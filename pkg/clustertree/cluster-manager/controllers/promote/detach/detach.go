package detach

import (
	"io"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/discovery"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/archive"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/filesystem"
)

// detach order, crd is detached first
var defaultDetachPriorities = []schema.GroupResource{
	kuberesource.CustomResourceDefinitions,
	kuberesource.StatefulSets,
	kuberesource.Deployments,
	kuberesource.ReplicaSets,
	kuberesource.PersistentVolumeClaims,
	kuberesource.Pods,
}

type kubernetesDetacher struct {
	request         *requests.PromoteRequest
	discoveryHelper discovery.Helper
	dynamicFactory  client.DynamicFactory // used for connect leaf cluster
	fileSystem      filesystem.Interface
	backupReader    io.Reader
	resourceClients map[resourceClientKey]client.Dynamic
	detachDir       string
	actions         map[string]DetachItemAction
}

func NewKubernetesDetacher(request *requests.PromoteRequest, backupReader io.Reader) (*kubernetesDetacher, error) {
	actions, err := registerDetachActions()
	if err != nil {
		return nil, err
	}
	dynamicFactory := client.NewDynamicFactory(request.LeafDynamicClient)
	discoveryHelper, err := discovery.NewHelper(request.LeafDiscoveryClient)
	if err != nil {
		return nil, err
	}

	return &kubernetesDetacher{
		request:         request,
		discoveryHelper: discoveryHelper,
		dynamicFactory:  dynamicFactory,
		fileSystem:      filesystem.NewFileSystem(),
		backupReader:    backupReader,
		resourceClients: make(map[resourceClientKey]client.Dynamic),
		actions:         actions,
	}, nil
}

// restoreableResource represents map of individual items of each resource
// identifier grouped by their original namespaces.
type detachableResource struct {
	resource                 string
	selectedItemsByNamespace map[string][]detachableItem
	totalItems               int
}

type detachableItem struct {
	path            string
	targetNamespace string
	name            string
	version         string // used for initializing informer cache
}

type resourceClientKey struct {
	resource  schema.GroupVersionResource
	namespace string
}

func (d *kubernetesDetacher) Detach() error {
	defer func() {
		// todo rollback if needed?
	}()

	dir, err := archive.NewExtractor(d.fileSystem).UnzipAndExtractBackup(d.backupReader)
	if err != nil {
		return errors.Errorf("error unzipping and extracting: %v", err)
	}
	defer func() {
		if err := d.fileSystem.RemoveAll(dir); err != nil {
			klog.Errorf("error removing temporary directory %s: %s", dir, err.Error())
		}
	}()

	// Need to set this for additionalItems to be restored.
	d.detachDir = dir

	backupResources, err := archive.NewParser(d.fileSystem).Parse(d.detachDir)
	if err != nil {
		return errors.Errorf("error parse detachDir %s: %v", d.detachDir, err)
	}
	klog.Infof("total backup resources size: %v", len(backupResources))

	resourceCollection, err := d.getOrderedResourceCollection(backupResources, defaultDetachPriorities)
	if err != nil {
		return err
	}

	for _, selectedResource := range resourceCollection {
		err = d.processSelectedResource(selectedResource)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *kubernetesDetacher) processSelectedResource(selectedResource detachableResource) error {
	groupResource := schema.ParseGroupResource(selectedResource.resource)

	for _, selectedItems := range selectedResource.selectedItemsByNamespace {
		for _, selectedItem := range selectedItems {
			obj, err := archive.Unmarshal(d.fileSystem, selectedItem.path)
			if err != nil {
				return errors.Errorf("error decoding %q: %v", strings.Replace(selectedItem.path, d.detachDir+"/", "", -1), err)
			}

			err = d.detachItem(obj, groupResource, selectedItem.targetNamespace)
			if err != nil {
				return errors.Wrap(err, "detachItem error")
			}

			key := requests.ItemKey{
				Resource:  groupResource.String(),
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}
			d.request.DetachedItems[key] = struct{}{}
		}
	}
	return nil
}

func (d *kubernetesDetacher) detachItem(obj *unstructured.Unstructured, groupResource schema.GroupResource, namespace string) error {
	resourceClient, err := d.getResourceClient(groupResource, obj, namespace)
	if err != nil {
		return errors.Wrap(err, "getResourceClient error")
	}

	if groupResource == kuberesource.StatefulSets || groupResource == kuberesource.Deployments || groupResource == kuberesource.ReplicaSets {
		//级联删除sts、deployment、replicaset等
		orphanOption := metav1.DeletePropagationOrphan
		if err = resourceClient.Delete(obj.GetName(), metav1.DeleteOptions{PropagationPolicy: &orphanOption}); err != nil {
			return errors.Wrap(err, "DeletePropagationOrphan err")
		}
	} else if action, ok := d.actions[groupResource.String()]; ok {
		err := action.Execute(obj, resourceClient)
		if err != nil {
			return errors.Errorf("%s detach action error: %v", groupResource.String(), err)
		}
	} else {
		// todo check if the gr provided was a custom resource
		customResource := false
		if customResource {
			updatedObj := obj.DeepCopy()
			res, ok := updatedObj.Object["metadata"]
			if !ok {
				return errors.New("metadata not found")
			}
			metadata, ok := res.(map[string]interface{})
			if !ok {
				return errors.Errorf("metadata was of type %T, expected map[string]interface{}", res)
			}

			if _, ok := metadata["finalizers"]; ok {
				delete(metadata, "finalizers")
				patchBytes, err := generatePatch(obj, updatedObj)
				if err != nil {
					return errors.Wrap(err, "error generating patch")
				}
				if patchBytes == nil {
					klog.Warningf("the same crd obj, %s", updatedObj.GetName())
				}

				klog.Infof("delete finalizers for %s", updatedObj.GetName())
				_, err = resourceClient.Patch(updatedObj.GetName(), patchBytes)
				if err != nil {
					return err
				}

				klog.Infof("delete cr %s", updatedObj.GetName())
				return resourceClient.Delete(updatedObj.GetName(), metav1.DeleteOptions{})
			}
		}
	}

	return nil
}

func (d *kubernetesDetacher) getOrderedResourceCollection(
	backupResources map[string]*archive.ResourceItems,
	groupResourcePriorities []schema.GroupResource,
) ([]detachableResource, error) {
	detachResourceCollection := make([]detachableResource, 20)

	for _, groupResource := range groupResourcePriorities {
		// try to resolve the resource via discovery to a complete group/version/resource
		_, _, err := d.discoveryHelper.ResourceFor(groupResource.WithVersion(""))
		if err != nil {
			klog.Infof("Skipping restore of resource %s because it cannot be resolved via discovery", groupResource.String())
			continue
		}

		// Check if the resource is present in the backup
		resourceList := backupResources[groupResource.String()]
		if resourceList == nil {
			klog.Infof("Skipping restore of resource %s because it's not present in the backup tarball", groupResource.String())
			continue
		}

		// Iterate through each namespace that contains instances of the
		// resource and append to the list of to-be restored resources.
		for namespace, items := range resourceList.ItemsByNamespace {
			res, err := d.getSelectedDetachableItems(groupResource.String(), namespace, items)
			if err != nil {
				return nil, err
			}

			detachResourceCollection = append(detachResourceCollection, res)
		}
	}
	return detachResourceCollection, nil
}

// getSelectedDetachableItems applies Kubernetes selectors on individual items
// of each resource type to create a list of items which will be actually
// restored.
func (d *kubernetesDetacher) getSelectedDetachableItems(resource string, namespace string, items []string) (detachableResource, error) {
	detachable := detachableResource{
		resource:                 resource,
		selectedItemsByNamespace: make(map[string][]detachableItem),
	}

	targetNamespace := namespace
	if targetNamespace != "" {
		klog.Infof("Resource '%s' will be restored into namespace '%s'", resource, targetNamespace)
	} else {
		klog.Infof("Resource '%s' will be restored at cluster scope", resource)
	}

	resourceForPath := resource

	for _, item := range items {
		itemPath := archive.GetItemFilePath(d.detachDir, resourceForPath, namespace, item)

		obj, err := archive.Unmarshal(d.fileSystem, itemPath)
		if err != nil {
			return detachable, errors.Errorf("error decoding %q: %v", strings.Replace(itemPath, d.detachDir+"/", "", -1), err)
		}

		if resource == kuberesource.Namespaces.String() {
			// handle remapping for namespace resource
			targetNamespace = item
		}

		selectedItem := detachableItem{
			path:            itemPath,
			name:            item,
			targetNamespace: targetNamespace,
			version:         obj.GroupVersionKind().Version,
		}
		detachable.selectedItemsByNamespace[namespace] =
			append(detachable.selectedItemsByNamespace[namespace], selectedItem)
		detachable.totalItems++
	}
	return detachable, nil
}

// generatePatch will calculate a JSON merge patch for an object's desired state.
// If the passed in objects are already equal, nil is returned.
func generatePatch(fromCluster, desired *unstructured.Unstructured) ([]byte, error) {
	// If the objects are already equal, there's no need to generate a patch.
	if equality.Semantic.DeepEqual(fromCluster, desired) {
		return nil, nil
	}

	desiredBytes, err := json.Marshal(desired.Object)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal desired object")
	}

	fromClusterBytes, err := json.Marshal(fromCluster.Object)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal in-cluster object")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(fromClusterBytes, desiredBytes)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create merge patch")
	}

	return patchBytes, nil
}

func (d *kubernetesDetacher) getResourceClient(groupResource schema.GroupResource, obj *unstructured.Unstructured, namespace string) (client.Dynamic, error) {
	key := resourceClientKey{
		resource:  groupResource.WithVersion(obj.GroupVersionKind().Version),
		namespace: namespace,
	}

	if client, ok := d.resourceClients[key]; ok {
		return client, nil
	}

	// Initialize client for this resource. We need metadata from an object to
	// do this.
	klog.Infof("Getting client for %v", obj.GroupVersionKind())

	resource := metav1.APIResource{
		Namespaced: len(namespace) > 0,
		Name:       groupResource.Resource,
	}

	clientForGroupVersionResource, err := d.dynamicFactory.ClientForGroupVersionResource(obj.GroupVersionKind().GroupVersion(), resource, namespace)
	if err != nil {
		return nil, err
	}

	d.resourceClients[key] = clientForGroupVersionResource
	return clientForGroupVersionResource, nil
}