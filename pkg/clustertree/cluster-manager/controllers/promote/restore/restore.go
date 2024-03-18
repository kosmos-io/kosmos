package restore

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/constants"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/discovery"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/types"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/archive"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/filesystem"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/kube"
)

/*
High priorities:
  - Custom Resource Definitions come before Custom Resource so that they can be
    restored with their corresponding CRD.
  - Namespaces go second because all namespaced resources depend on them.
  - Storage Classes are needed to create PVs and PVCs correctly.
  - VolumeSnapshotClasses  are needed to provision volumes using volumesnapshots
  - VolumeSnapshotContents are needed as they contain the handle to the volume snapshot in the
    storage provider
  - VolumeSnapshots are needed to create PVCs using the VolumeSnapshot as their data source.
  - DataUploads need to restore before PVC for Snapshot DataMover to work, because PVC needs the DataUploadResults to create DataDownloads.
  - PVs go before PVCs because PVCs depend on them.
  - PVCs go before pods or controllers so they can be mounted as volumes.
  - Service accounts go before secrets so service account token secrets can be filled automatically.
  - Secrets and ConfigMaps go before pods or controllers so they can be mounted
    as volumes.
  - Limit ranges go before pods or controllers so pods can use them.
  - Pods go before controllers so they can be explicitly restored and potentially
    have pod volume restores run before controllers adopt the pods.
  - Replica sets go before deployments/other controllers so they can be explicitly
    restored and be adopted by controllers.
  - Services go before Clusters so they can be adopted by AKO-operator and no new Services will be created
    for the same clusters
*/
var defaultRestorePriorities = Priorities{
	HighPriorities: []string{
		"customresourcedefinitions",
		"namespaces",
		"persistentvolumeclaims",
		"persistentvolumes",
		"serviceaccounts",
		"roles.rbac.authorization.k8s.io",
		"rolebindings.rbac.authorization.k8s.io",
		"secrets",
		"configmaps",
		"pods",
		"replicasets.apps",
		"deployments.apps",
		"statefulsets.apps",
		"services",
	},
	LowPriorities: []string{},
}

// kubernetesRestorer implements Restorer for restoring into a Kubernetes cluster.
type kubernetesRestorer struct {
	request                    *requests.PromoteRequest
	discoveryHelper            discovery.Helper
	dynamicFactory             client.DynamicFactory
	fileSystem                 filesystem.Interface
	restoreDir                 string
	actions                    map[string]RestoreItemAction
	namespaceClient            corev1.NamespaceInterface
	resourceClients            map[resourceClientKey]client.Dynamic
	resourceTerminatingTimeout time.Duration
	backupReader               io.Reader
	kosmosClusterName          string
	kosmosNodeName             string
}

// restoreableResource represents map of individual items of each resource
// identifier grouped by their original namespaces.
type restoreableResource struct {
	resource                 string
	selectedItemsByNamespace map[string][]restoreableItem
	totalItems               int
}

// restoreableItem represents an item by its target namespace contains enough
// information required to restore the item.
type restoreableItem struct {
	path            string
	targetNamespace string
	name            string
	version         string // used for initializing informer cache
}

type resourceClientKey struct {
	resource  schema.GroupVersionResource
	namespace string
}

func NewKubernetesRestorer(request *requests.PromoteRequest, backupReader io.Reader) (*kubernetesRestorer, error) {
	dynamicFactory := client.NewDynamicFactory(request.RootDynamicClient)
	discoveryHelper, err := discovery.NewHelper(request.RootDiscoveryClient)
	if err != nil {
		return nil, err
	}

	actions, err := registerRestoreActions()
	if err != nil {
		return nil, err
	}
	return &kubernetesRestorer{
		request:                    request,
		discoveryHelper:            discoveryHelper,
		dynamicFactory:             dynamicFactory,
		namespaceClient:            request.RootClientSet.CoreV1().Namespaces(),
		resourceTerminatingTimeout: 10 * time.Minute,
		fileSystem:                 filesystem.NewFileSystem(),
		backupReader:               backupReader,
		kosmosClusterName:          request.Spec.ClusterName,
		kosmosNodeName:             "kosmos-" + request.Spec.ClusterName,
		resourceClients:            make(map[resourceClientKey]client.Dynamic),
		actions:                    actions,
	}, nil
}

func (kr *kubernetesRestorer) Restore() error {
	klog.Infof("Starting restore of backup")

	defer func() {
		// todo rollback if needed?
	}()

	dir, err := archive.NewExtractor(kr.fileSystem).UnzipAndExtractBackup(kr.backupReader)
	if err != nil {
		return errors.Errorf("error unzipping and extracting: %v", err)
	}
	defer func() {
		if err := kr.fileSystem.RemoveAll(dir); err != nil {
			klog.Errorf("error removing temporary directory %s: %s", dir, err.Error())
		}
	}()

	// Need to set this for additionalItems to be restored.
	kr.restoreDir = dir

	backupResources, err := archive.NewParser(kr.fileSystem).Parse(kr.restoreDir)
	// If ErrNotExist occurs, it implies that the backup to be restored includes zero items.
	// Need to add a warning about it and jump out of the function.
	if errors.Cause(err) == archive.ErrNotExist {
		return errors.Wrap(err, "zero items to be restored")
	}
	if err != nil {
		return errors.Wrap(err, "error parsing backup contents")
	}

	klog.Infof("total backup resources size: %v", len(backupResources))

	// totalItems: previously discovered items, i: iteration counter.
	processedItems, existingNamespaces := 0, sets.KeySet(make(map[string]struct{}))

	klog.Infof("Restore everything order by defaultRestorePriorities")
	// Restore everything else
	selectedResourceCollection, _, err := kr.getOrderedResourceCollection(
		backupResources,
		make([]restoreableResource, 0),
		sets.KeySet(make(map[string]string)),
		defaultRestorePriorities,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "getOrderedResourceCollection err")
	}

	klog.Infof("resource collection size: %s", len(selectedResourceCollection))

	for _, selectedResource := range selectedResourceCollection {
		// Restore this resource
		processedItems, err = kr.processSelectedResource(
			selectedResource,
			processedItems,
			existingNamespaces,
		)
		if err != nil {
			return errors.Wrap(err, "processSelectedResource err")
		}
	}

	return nil
}

func (kr *kubernetesRestorer) Rollback(allRestored bool) error {
	dir, err := archive.NewExtractor(kr.fileSystem).UnzipAndExtractBackup(kr.backupReader)
	if err != nil {
		return errors.Errorf("error unzipping and extracting: %v", err)
	}
	defer func() {
		if err := kr.fileSystem.RemoveAll(dir); err != nil {
			klog.Errorf("error removing temporary directory %s: %s", dir, err.Error())
		}
	}()

	// Need to set this for additionalItems to be restored.
	kr.restoreDir = dir

	backupResources, err := archive.NewParser(kr.fileSystem).Parse(kr.restoreDir)
	// If ErrNotExist occurs, it implies that the backup to be restored includes zero items.
	// Need to add a warning about it and jump out of the function.
	if errors.Cause(err) == archive.ErrNotExist {
		return errors.Wrap(err, "zero items to be restored")
	}
	if err != nil {
		return errors.Wrap(err, "error parsing backup contents")
	}

	klog.Infof("total backup resources size: %v", len(backupResources))

	var highProprites []string
	highProprites = append(highProprites, defaultRestorePriorities.HighPriorities...)
	reversSlice(highProprites)
	unestorePriorities := Priorities{
		HighPriorities: highProprites,
		LowPriorities:  defaultRestorePriorities.LowPriorities,
	}

	selectedResourceCollection, _, err := kr.getOrderedResourceCollection(
		backupResources,
		make([]restoreableResource, 0),
		sets.KeySet(make(map[string]string)),
		unestorePriorities,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "getOrderedResourceCollection err")
	}

	for _, selectedResource := range selectedResourceCollection {
		// Restore this resource
		err = kr.deleteSelectedResource(selectedResource, allRestored)
		if err != nil {
			return errors.Wrap(err, "deleteSelectedResource err")
		}
	}

	return nil
}

// getOrderedResourceCollection iterates over list of ordered resource
// identifiers, applies resource include/exclude criteria, and Kubernetes
// selectors to make a list of resources to be actually restored preserving the
// original order.
func (kr *kubernetesRestorer) getOrderedResourceCollection(
	backupResources map[string]*archive.ResourceItems,
	restoreResourceCollection []restoreableResource,
	processedResources sets.Set[string],
	resourcePriorities Priorities,
	includeAllResources bool,
) ([]restoreableResource, sets.Set[string], error) {
	var resourceList []string
	if includeAllResources {
		resourceList = getOrderedResources(resourcePriorities, backupResources)
	} else {
		resourceList = resourcePriorities.HighPriorities
	}

	for _, resource := range resourceList {
		// try to resolve the resource via discovery to a complete group/version/resource
		gvr, _, err := kr.discoveryHelper.ResourceFor(schema.ParseGroupResource(resource).WithVersion(""))
		if err != nil {
			klog.Infof("Skipping restore of resource %s because it cannot be resolved via discovery", resource)
			continue
		}
		groupResource := gvr.GroupResource()

		// Check if we've already restored this resource (this would happen if
		// the resource we're currently looking at was already restored because
		// it was a prioritized resource, and now we're looking at it as part of
		// the backup contents).
		if processedResources.Has(groupResource.String()) {
			klog.Infof("Skipping restore of resource %s because it's already been processed", groupResource.String())
			continue
		}

		// Check if the resource should be restored according to the resource
		// includes/excludes.

		// Check if the resource is present in the backup
		resourceList := backupResources[groupResource.String()]
		if resourceList == nil {
			klog.Infof("Skipping restore of resource %s because it's not present in the backup tarball", groupResource.String())
			continue
		}

		// Iterate through each namespace that contains instances of the
		// resource and append to the list of to-be restored resources.
		for namespace, items := range resourceList.ItemsByNamespace {
			res, err := kr.getSelectedRestoreableItems(groupResource.String(), namespace, items)
			if err != nil {
				return nil, nil, errors.Wrap(err, "getSelectedRestoreableItems err")
			}

			restoreResourceCollection = append(restoreResourceCollection, res)
		}

		// record that we've restored the resource
		processedResources.Insert(groupResource.String())
	}
	return restoreResourceCollection, processedResources, nil
}

// Process and restore one restoreableResource from the backup and update restore progress
// metadata. At this point, the resource has already been validated and counted for inclusion
// in the expected total restore count.
func (kr *kubernetesRestorer) processSelectedResource(
	selectedResource restoreableResource,
	processedItems int,
	existingNamespaces sets.Set[string],
) (int, error) {
	groupResource := schema.ParseGroupResource(selectedResource.resource)

	for namespace, selectedItems := range selectedResource.selectedItemsByNamespace {
		for _, selectedItem := range selectedItems {
			if groupResource == kuberesource.Namespaces {
				namespace = selectedItem.name
			}

			// If we don't know whether this namespace exists yet, attempt to create
			// it in order to ensure it exists. Try to get it from the backup tarball
			// (in order to get any backed-up metadata), but if we don't find it there,
			// create a blank one.
			if namespace != "" && !existingNamespaces.Has(selectedItem.targetNamespace) {
				ns := getNamespace(
					archive.GetItemFilePath(kr.restoreDir, "namespaces", "", namespace),
					selectedItem.targetNamespace,
				)
				_, nsCreated, err := kube.EnsureNamespaceExistsAndIsReady(
					ns,
					kr.namespaceClient,
					kr.resourceTerminatingTimeout,
				)
				if err != nil {
					return processedItems, err
				}

				// Add the newly created namespace to the list of restored items.
				if nsCreated {
					itemKey := types.ItemKey{
						Resource:  groupResource.String(),
						Namespace: ns.Namespace,
						Name:      ns.Name,
					}
					kr.request.RestoredItems[itemKey] = types.RestoredItemStatus{Action: constants.ItemRestoreResultCreated, ItemExists: true}
				}

				// Keep track of namespaces that we know exist so we don't
				// have to try to create them multiple times.
				existingNamespaces.Insert(selectedItem.targetNamespace)
			}

			// For namespaces resources we don't need to following steps
			if groupResource == kuberesource.Namespaces {
				continue
			}

			obj, err := archive.Unmarshal(kr.fileSystem, selectedItem.path)
			if err != nil {
				if err != nil {
					return processedItems, errors.Errorf("error decoding %q: %v", strings.Replace(selectedItem.path, kr.restoreDir+"/", "", -1), err)
				}
			}

			_, err = kr.restoreItem(obj, groupResource, selectedItem.targetNamespace)
			if err != nil {
				return processedItems, errors.Wrap(err, "restoreItem error")
			}
			processedItems++
		}
	}

	return processedItems, nil
}

func (kr *kubernetesRestorer) deleteSelectedResource(selectedResource restoreableResource, allRestored bool) error {
	groupResource := schema.ParseGroupResource(selectedResource.resource)

	for _, selectedItems := range selectedResource.selectedItemsByNamespace {
		for _, selectedItem := range selectedItems {
			obj, err := archive.Unmarshal(kr.fileSystem, selectedItem.path)
			if err != nil {
				if err != nil {
					return errors.Errorf("error decoding %q: %v", strings.Replace(selectedItem.path, kr.restoreDir+"/", "", -1), err)
				}
			}

			if !allRestored {
				item := types.ItemKey{
					Resource:  groupResource.String(),
					Name:      selectedItem.name,
					Namespace: selectedItem.targetNamespace,
				}

				if _, ok := kr.request.RestoredItems[item]; !ok {
					// unrestored resource, doesn't need to handle
					continue
				}
			}

			_, err = kr.deleteItem(obj, groupResource, selectedItem.targetNamespace)
			if err != nil {
				return errors.Wrap(err, "deleteItem error")
			}
		}
	}

	return nil
}

// getSelectedRestoreableItems applies Kubernetes selectors on individual items
// of each resource type to create a list of items which will be actually
// restored.
func (kr *kubernetesRestorer) getSelectedRestoreableItems(resource string, namespace string, items []string) (restoreableResource, error) {
	restorable := restoreableResource{
		resource:                 resource,
		selectedItemsByNamespace: make(map[string][]restoreableItem),
	}

	targetNamespace := namespace
	if targetNamespace != "" {
		klog.Infof("Resource '%s' will be restored into namespace '%s'", resource, targetNamespace)
	} else {
		klog.Infof("Resource '%s' will be restored at cluster scope", resource)
	}

	resourceForPath := resource

	for _, item := range items {
		itemPath := archive.GetItemFilePath(kr.restoreDir, resourceForPath, namespace, item)

		obj, err := archive.Unmarshal(kr.fileSystem, itemPath)
		if err != nil {
			return restorable, errors.Errorf("error decoding %q: %v", strings.Replace(itemPath, kr.restoreDir+"/", "", -1), err)
		}

		if resource == kuberesource.Namespaces.String() {
			// handle remapping for namespace resource
			targetNamespace = item
		}

		selectedItem := restoreableItem{
			path:            itemPath,
			name:            item,
			targetNamespace: targetNamespace,
			version:         obj.GroupVersionKind().Version,
		}
		restorable.selectedItemsByNamespace[namespace] =
			append(restorable.selectedItemsByNamespace[namespace], selectedItem)
		restorable.totalItems++
	}
	return restorable, nil
}

func (kr *kubernetesRestorer) restoreItem(obj *unstructured.Unstructured, groupResource schema.GroupResource, namespace string) (bool, error) {
	// itemExists bool is used to determine whether to include this item in the "wait for additional items" list
	itemExists := false
	resourceID := getResourceID(groupResource, namespace, obj.GetName())

	if namespace != "" {
		nsToEnsure := getNamespace(archive.GetItemFilePath(kr.restoreDir, "namespaces", "", obj.GetNamespace()), namespace)
		_, nsCreated, err := kube.EnsureNamespaceExistsAndIsReady(nsToEnsure, kr.namespaceClient, kr.resourceTerminatingTimeout)
		if err != nil {
			return itemExists, err
		}
		// Add the newly created namespace to the list of restored items.
		if nsCreated {
			itemKey := types.ItemKey{
				Resource:  groupResource.String(),
				Namespace: nsToEnsure.Namespace,
				Name:      nsToEnsure.Name,
			}
			kr.request.RestoredItems[itemKey] = types.RestoredItemStatus{Action: constants.ItemRestoreResultCreated, ItemExists: true}
		}
	}

	complete, err := isCompleted(obj, groupResource)
	if err != nil {
		return itemExists, errors.Errorf("error checking completion of %q: %v", resourceID, err)
	}
	if complete {
		klog.Infof("%s is complete - skipping", kube.NamespaceAndName(obj))
		return itemExists, nil
	}

	name := obj.GetName()

	// Check if we've already restored this itemKey.
	itemKey := types.ItemKey{
		Resource:  groupResource.String(),
		Namespace: namespace,
		Name:      name,
	}

	if prevRestoredItemStatus, exists := kr.request.RestoredItems[itemKey]; exists {
		klog.Infof("Skipping %s because it's already been restored.", resourceID)
		itemExists = prevRestoredItemStatus.ItemExists
		return itemExists, nil
	}
	kr.request.RestoredItems[itemKey] = types.RestoredItemStatus{ItemExists: itemExists}
	defer func() {
		itemStatus := kr.request.RestoredItems[itemKey]
		// the action field is set explicitly
		if len(itemStatus.Action) > 0 {
			return
		}
		// others are all failed
		itemStatus.Action = constants.ItemRestoreResultFailed
		kr.request.RestoredItems[itemKey] = itemStatus
	}()

	if action, ok := kr.actions[groupResource.String()]; ok {
		obj, err = action.Execute(obj, kr)
		if err != nil {
			return itemExists, errors.Errorf("error execute %s action: %v", groupResource.String(), err)
		}
	}

	//objStatus, statusFieldExists, statusFieldErr := unstructured.NestedFieldCopy(obj.Object, "status")
	// Clear out non-core metadata fields and status.
	if obj, err = kube.ResetMetadataAndStatus(obj); err != nil {
		return itemExists, err
	}

	// The object apiVersion might get modified by a RestorePlugin so we need to
	// get a new client to reflect updated resource path.
	newGR := schema.GroupResource{Group: obj.GroupVersionKind().Group, Resource: groupResource.Resource}
	resourceClient, err := kr.getResourceClient(newGR, obj, obj.GetNamespace())
	if err != nil {
		return itemExists, errors.Errorf("error getting updated resource client for namespace %q, resource %q: %v", namespace, &groupResource, err)
	}

	klog.Infof("Attempting to restore %s: %v", obj.GroupVersionKind().Kind, name)

	var _ *unstructured.Unstructured
	var restoreErr error

	klog.Infof("Creating %s: %v", obj.GroupVersionKind().Kind, name)
	_, restoreErr = resourceClient.Create(obj)
	if restoreErr == nil {
		itemExists = true
		kr.request.RestoredItems[itemKey] = types.RestoredItemStatus{Action: constants.ItemRestoreResultCreated, ItemExists: itemExists}
	}

	// Error was something other than an AlreadyExists.
	if restoreErr != nil {
		if apierrors.IsAlreadyExists(restoreErr) {
			klog.Warningf("%s already exists", resourceID)
			return itemExists, nil
		}
		return itemExists, errors.Errorf("error restoring %s: %v", resourceID, restoreErr)
	}

	return itemExists, nil
}

func (kr *kubernetesRestorer) deleteItem(obj *unstructured.Unstructured, groupResource schema.GroupResource, namespace string) (bool, error) {
	// Check if we've already restored this itemKey.
	itemKey := types.ItemKey{
		Resource:  groupResource.String(),
		Namespace: namespace,
		Name:      obj.GetName(),
	}

	// The object apiVersion might get modified by a RestorePlugin so we need to
	// get a new client to reflect updated resource path.
	resourceClient, err := kr.getResourceClient(groupResource, obj, obj.GetNamespace())
	if err != nil {
		return false, errors.Errorf("error getting updated resource client for namespace %q, resource %q: %v", namespace, &groupResource, err)
	}

	if action, ok := kr.actions[groupResource.String()]; ok {
		klog.Infof("Attempting to revert %s: %v", obj.GroupVersionKind().Kind, obj.GetName())
		fromCluster, err := resourceClient.Get(obj.GetName(), metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Warningf("resource %s not found. skip unstore", obj.GroupVersionKind().String(), obj.GetName())
				return true, nil
			} else {
				return false, errors.Wrapf(err, "get resource %s %s failed.", obj.GroupVersionKind().String(), obj.GetName())
			}
		}

		updatedObj, err := action.Revert(fromCluster, kr)
		if err != nil {
			return false, errors.Errorf("error revert %s action: %v", groupResource.String(), err)
		}

		patchBytes, err := kube.GeneratePatch(fromCluster, updatedObj)
		if err != nil {
			return false, errors.Wrap(err, "error generating patch")
		}
		if patchBytes == nil {
			klog.Infof("the same obj %s. skipped patch", obj.GetName())
		} else {
			_, err = resourceClient.Patch(obj.GetName(), patchBytes)
			if err != nil {
				return false, errors.Wrapf(err, "patch %s error", obj.GetName())
			}
		}
	}

	klog.Infof("Deleting %s: %v", obj.GroupVersionKind().Kind, obj.GetName())
	deleteOptions := metav1.DeleteOptions{}
	if groupResource == kuberesource.Pods {
		graceDeleteSecond := int64(0)
		deleteOptions = metav1.DeleteOptions{GracePeriodSeconds: &graceDeleteSecond}
	}
	err = resourceClient.Delete(obj.GetName(), deleteOptions)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("delete %s %s error because resource not found.", obj.GroupVersionKind().String(), obj.GetName())
		} else {
			klog.Errorf("error delete delete %s %s. %s", obj.GroupVersionKind().String(), obj.GetName(), err.Error())
		}
	}
	delete(kr.request.RestoredItems, itemKey)
	return true, nil
}

func (kr *kubernetesRestorer) getResourceClient(groupResource schema.GroupResource, obj *unstructured.Unstructured, namespace string) (client.Dynamic, error) {
	key := getResourceClientKey(groupResource, obj.GroupVersionKind().Version, namespace)

	if client, ok := kr.resourceClients[key]; ok {
		return client, nil
	}

	// Initialize client for this resource. We need metadata from an object to
	// do this.
	klog.Infof("Getting client for %v", obj.GroupVersionKind())

	resource := metav1.APIResource{
		Namespaced: len(namespace) > 0,
		Name:       groupResource.Resource,
	}

	client, err := kr.dynamicFactory.ClientForGroupVersionResource(obj.GroupVersionKind().GroupVersion(), resource, namespace)
	if err != nil {
		return nil, err
	}

	kr.resourceClients[key] = client
	return client, nil
}
func getResourceClientKey(groupResource schema.GroupResource, version, namespace string) resourceClientKey {
	return resourceClientKey{
		resource:  groupResource.WithVersion(version),
		namespace: namespace,
	}
}

// isCompleted returns whether or not an object is considered completed. Used to
// identify whether or not an object should be restored. Only Jobs or Pods are
// considered.
func isCompleted(obj *unstructured.Unstructured, groupResource schema.GroupResource) (bool, error) {
	switch groupResource {
	case kuberesource.Pods:
		phase, _, err := unstructured.NestedString(obj.UnstructuredContent(), "status", "phase")
		if err != nil {
			return false, errors.WithStack(err)
		}
		if phase == string(v1.PodFailed) || phase == string(v1.PodSucceeded) {
			return true, nil
		}

	case kuberesource.Jobs:
		ct, found, err := unstructured.NestedString(obj.UnstructuredContent(), "status", "completionTime")
		if err != nil {
			return false, errors.WithStack(err)
		}
		if found && ct != "" {
			return true, nil
		}
	}
	// Assume any other resource isn't complete and can be restored.
	return false, nil
}

func getResourceID(groupResource schema.GroupResource, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", groupResource.String(), name)
	}

	return fmt.Sprintf("%s/%s/%s", groupResource.String(), namespace, name)
}

// getNamespace returns a namespace API object that we should attempt to
// create before restoring anything into it. It will come from the backup
// tarball if it exists, else will be a new one. If from the tarball, it
// will retain its labels, annotations, and spec.
func getNamespace(path, remappedName string) *v1.Namespace {
	var nsBytes []byte
	var err error

	if nsBytes, err = os.ReadFile(path); err != nil {
		return &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: remappedName,
			},
		}
	}

	var backupNS v1.Namespace
	if err := json.Unmarshal(nsBytes, &backupNS); err != nil {
		klog.Warningf("Error unmarshaling namespace from backup, creating new one.")
		return &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: remappedName,
			},
		}
	}

	return &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       backupNS.Kind,
			APIVersion: backupNS.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        remappedName,
			Labels:      backupNS.Labels,
			Annotations: backupNS.Annotations,
		},
		Spec: backupNS.Spec,
	}
}

// getOrderedResources returns an ordered list of resource identifiers to restore,
// based on the provided resource priorities and backup contents. The returned list
// begins with all of the high prioritized resources (in order), ends with all of
// the low prioritized resources(in order), and an alphabetized list of resources
// in the backup(pick out the prioritized resources) is put in the middle.
func getOrderedResources(resourcePriorities Priorities, backupResources map[string]*archive.ResourceItems) []string {
	priorities := map[string]struct{}{}
	for _, priority := range resourcePriorities.HighPriorities {
		priorities[priority] = struct{}{}
	}
	for _, priority := range resourcePriorities.LowPriorities {
		priorities[priority] = struct{}{}
	}

	// pick the prioritized resources out
	var orderedBackupResources []string
	for resource := range backupResources {
		if _, exist := priorities[resource]; exist {
			continue
		}
		orderedBackupResources = append(orderedBackupResources, resource)
	}
	// alphabetize resources in the backup
	sort.Strings(orderedBackupResources)

	list := append(resourcePriorities.HighPriorities, orderedBackupResources...)
	return append(list, resourcePriorities.LowPriorities...)
}

// ReversSlice reverse the slice
func reversSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
