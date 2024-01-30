package backup

import (
	"os"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/discovery"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/collections"
)

// itemCollector collects items from the Kubernetes API according to
// the backup spec and writes them to files inside dir.
type itemCollector struct {
	request         *requests.PromoteRequest
	discoveryHelper discovery.Helper
	dynamicFactory  client.DynamicFactory
	dir             string
}

type kubernetesResource struct {
	groupResource         schema.GroupResource
	preferredGVR          schema.GroupVersionResource
	namespace, name, path string
}

// These constants represent the relative priorities for resources in the core API group. We want to
// ensure that we process pods, then pvcs, then pvs, then anything else. This ensures that when a
// pod is backed up, we can perform a pre hook, then process pvcs and pvs (including taking a
// snapshot), then perform a post hook on the pod.
const (
	pod = iota
	pvc
	pv
	other
)

// getAllItems gets all relevant items from all API groups.
func (r *itemCollector) getAllItems() []*kubernetesResource {
	var resources []*kubernetesResource
	for _, group := range r.discoveryHelper.Resources() {
		groupItems, err := r.getGroupItems(group)
		if err != nil {
			klog.Errorf("Error collecting resources from API group %s. %v", group.String(), err)
			continue
		}

		resources = append(resources, groupItems...)
	}

	return resources
}

// getGroupItems collects all relevant items from a single API group.
func (r *itemCollector) getGroupItems(group *metav1.APIResourceList) ([]*kubernetesResource, error) {
	klog.Infof("Getting items for group %s", group.GroupVersion)

	// Parse so we can check if this is the core group
	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing GroupVersion %q", group.GroupVersion)
	}
	if gv.Group == "" {
		// This is the core group, so make sure we process in the following order: pods, pvcs, pvs, else
		sortCoreGroup(group)
	}

	var items []*kubernetesResource
	for _, resource := range group.APIResources {
		resourceItems, err := r.getResourceItems(gv, resource)
		if err != nil {
			klog.Errorf("Error getting items for resource %s", resource.String())
			continue
		}

		items = append(items, resourceItems...)
	}

	return items, nil
}

// getResourceItems collects all relevant items for a given group-version-resource.
func (r *itemCollector) getResourceItems(gv schema.GroupVersion, resource metav1.APIResource) ([]*kubernetesResource, error) {
	klog.Info("Getting items for resource %s", resource.Name)

	var (
		gvr = gv.WithResource(resource.Name)
		gr  = gvr.GroupResource()
	)

	//orders := getOrderedResourcesForType(r.backupRequest.Backup.Spec.OrderedResources, resource.Name)

	// Getting the preferred group version of this resource
	preferredGVR, _, err := r.discoveryHelper.ResourceFor(gr.WithVersion(""))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if !r.request.ResourceIncludesExcludes.ShouldInclude(gr.String()) {
		klog.Infof("Skipping resource %s because it's excluded", gr.String())
		return nil, nil
	}

	//if cohabitator, found := r.cohabitatingResources[resource.Name]; found {
	//	if gv.Group == cohabitator.groupResource1.Group || gv.Group == cohabitator.groupResource2.Group {
	//		if cohabitator.seen {
	//			log.WithFields(
	//				logrus.Fields{
	//					"cohabitatingResource1": cohabitator.groupResource1.String(),
	//					"cohabitatingResource2": cohabitator.groupResource2.String(),
	//				},
	//			).Infof("Skipping resource because it cohabitates and we've already processed it")
	//			return nil, nil
	//		}
	//		cohabitator.seen = true
	//	}
	//}

	// Handle namespace resource here
	if gr == kuberesource.Namespaces {
		resourceClient, err := r.dynamicFactory.ClientForGroupVersionResource(gv, resource, "")
		if err != nil {
			klog.Errorf("Error getting dynamic client. %v", errors.WithStack(err))
			return nil, errors.WithStack(err)
		}
		unstructuredList, err := resourceClient.List(metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Error list namespaces. %v", errors.WithStack(err))
			return nil, errors.WithStack(err)
		}

		items := r.backupNamespaces(unstructuredList, r.request.NamespaceIncludesExcludes, gr, preferredGVR)

		return items, nil
	}

	clusterScoped := !resource.Namespaced
	namespacesToList := getNamespacesToList(r.request.NamespaceIncludesExcludes)

	if clusterScoped {
		//namespacesToList = []string{""}
		return nil, nil
	}

	var items []*kubernetesResource

	for _, namespace := range namespacesToList {
		// List items from kubernetes API

		resourceClient, err := r.dynamicFactory.ClientForGroupVersionResource(gv, resource, namespace)
		if err != nil {
			klog.Errorf("Error getting dynamic client. %v", err)
			continue
		}

		var orLabelSelectors []string
		//if r.backupRequest.Spec.OrLabelSelectors != nil {
		//	for _, s := range r.backupRequest.Spec.OrLabelSelectors {
		//		orLabelSelectors = append(orLabelSelectors, metav1.FormatLabelSelector(s))
		//	}
		//} else {
		//	orLabelSelectors = []string{}
		//}

		unstructuredItems := make([]unstructured.Unstructured, 0)

		// Listing items for orLabelSelectors
		//errListingForNS := false
		//for _, label := range orLabelSelectors {
		//	unstructuredItems, err = r.listItemsForLabel(unstructuredItems, gr, label, resourceClient)
		//	if err != nil {
		//		errListingForNS = true
		//	}
		//}

		//if errListingForNS {
		//	log.WithError(err).Error("Error listing items")
		//	continue
		//}

		var labelSelector string
		//if selector := r.backupRequest.Spec.LabelSelector; selector != nil {
		//	labelSelector = metav1.FormatLabelSelector(selector)
		//}

		// Listing items for labelSelector (singular)
		if len(orLabelSelectors) == 0 {
			unstructuredItems, err = r.listItemsForLabel(unstructuredItems, gr, labelSelector, resourceClient)
			if err != nil {
				klog.Errorf("Error listing items. %v", err)
				continue
			}
		}

		// Collect items in included Namespaces
		for i := range unstructuredItems {
			item := &unstructuredItems[i]

			path, err := r.writeToFile(item)
			if err != nil {
				klog.Errorf("Error writing item to file. %v", err)
				continue
			}

			items = append(items, &kubernetesResource{
				groupResource: gr,
				preferredGVR:  preferredGVR,
				namespace:     item.GetNamespace(),
				name:          item.GetName(),
				path:          path,
			})
		}
	}

	//if len(orders) > 0 {
	//	items = sortResourcesByOrder(r.log, items, orders)
	//}

	return items, nil
}

func (r *itemCollector) listItemsForLabel(unstructuredItems []unstructured.Unstructured, gr schema.GroupResource, label string, resourceClient client.Dynamic) ([]unstructured.Unstructured, error) {
	unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: label})
	if err != nil {
		klog.Errorf("Error listing items. %v", errors.WithStack(err))
		return unstructuredItems, err
	}
	unstructuredItems = append(unstructuredItems, unstructuredList.Items...)
	return unstructuredItems, nil
}

// backupNamespaces process namespace resource according to namespace filters.
func (r *itemCollector) backupNamespaces(unstructuredList *unstructured.UnstructuredList, ie *collections.IncludesExcludes,
	gr schema.GroupResource, preferredGVR schema.GroupVersionResource) []*kubernetesResource {
	var items []*kubernetesResource
	for index, unstructured := range unstructuredList.Items {
		if ie.ShouldInclude(unstructured.GetName()) {
			klog.Infof("Backup namespace %s.", unstructured.GetName())

			path, err := r.writeToFile(&unstructuredList.Items[index])
			if err != nil {
				klog.Errorf("Error writing item to file. %v", err)
				continue
			}

			items = append(items, &kubernetesResource{
				groupResource: gr,
				preferredGVR:  preferredGVR,
				name:          unstructured.GetName(),
				path:          path,
			})
		}
	}

	return items
}

func (r *itemCollector) writeToFile(item *unstructured.Unstructured) (string, error) {
	logrus.Infof("dir path: %s", r.dir)
	f, err := os.CreateTemp(r.dir, "")
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file")
	}
	defer f.Close()

	jsonBytes, err := json.Marshal(item)
	if err != nil {
		return "", errors.Wrap(err, "error convering item to JSON")
	}

	if _, err := f.Write(jsonBytes); err != nil {
		return "", errors.Wrap(err, "error writing JSON to file")
	}

	if err := f.Close(); err != nil {
		return "", errors.Wrap(err, "error closing file")
	}

	return f.Name(), nil
}

// SortCoreGroup sorts the core API group
func sortCoreGroup(group *metav1.APIResourceList) {
	sort.SliceStable(group.APIResources, func(i, j int) bool {
		return coreGroupResourcePriority(group.APIResources[i].Name) < coreGroupResourcePriority(group.APIResources[j].Name)
	})
}

func coreGroupResourcePriority(resource string) int {
	switch strings.ToLower(resource) {
	case "pods":
		return pod
	case "persistentvolumeclaims":
		return pvc
	case "persistentvolumes":
		return pv
	}

	return other
}

func getNamespacesToList(ie *collections.IncludesExcludes) []string {
	if ie == nil {
		return []string{""}
	}

	var list []string
	for _, i := range ie.GetIncludes() {
		if ie.ShouldInclude(i) {
			list = append(list, i)
		}
	}

	return list
}
