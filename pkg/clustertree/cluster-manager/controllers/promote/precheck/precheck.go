package precheck

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils"
	constants "github.com/kosmos.io/kosmos/pkg/utils"
)

type kubernetesPrecheck struct {
	request *requests.PromoteRequest
}

func NewKubernetesPrecheck(request *requests.PromoteRequest) (*kubernetesPrecheck, error) {
	if request != nil {
		return &kubernetesPrecheck{request: request}, nil
	} else {
		return nil, fmt.Errorf("request is nil")
	}
}
func (kb *kubernetesPrecheck) Precheck() error {
	// check namespace
	err := checkNamespaces(kb.request, kb.request.ForbidNamespaces)
	if err != nil {
		return err
	}

	// check ApiResources
	err = checkApiResources(kb.request)
	if err != nil {
		return err
	}

	return nil
}

func checkApiResources(request *requests.PromoteRequest) error {
	// judge k8s version
	leafVersion, err := request.LeafDiscoveryClient.ServerVersion()
	if err != nil {
		return err
	}
	rootVersion, err := request.RootDiscoveryClient.ServerVersion()
	if err != nil {
		return err
	}
	if !strings.EqualFold(leafVersion.GitVersion, rootVersion.GitVersion) {
		return fmt.Errorf("kubernetes version is not same in leaf cluster and rootcluster")
	}

	includedResources := request.Spec.IncludedNamespaceScopedResources
	excludedResources := request.Spec.ExcludedNamespaceScopedResources

	for _, excludedResource := range excludedResources {
		if excludedResource == "*" {
			return fmt.Errorf("precheck failed, excluded resources has \"*\" ")
		}
	}

	for _, includedResource := range includedResources {
		// add all resources to includedResources
		if includedResource == "*" {
			// gets all preferred api resources for the leaf cluster
			leafApiResourcesMap, err := getApiResourcesMap(request.LeafClientSet)
			if err != nil {
				return fmt.Errorf("precheck failed, getApiResourcesMap in leaf cluster fauled, err: %s", err)
			}
			var tmp []string
			for name := range leafApiResourcesMap {
				tmp = append(tmp, name)
			}
			includedResources = tmp
			break
		}
	}

	// needsStringMap is excludedResources converted into map
	excludeMap, err := utils.ToMapSetE(excludedResources)
	if err != nil {
		return fmt.Errorf("includedResources convert to map failed, err: %s", err)
	}
	excludeStringMap := make(map[string]string)
	for _, value := range excludeMap.(map[interface{}]interface{}) {
		valueString := value.(string)
		excludeStringMap[valueString] = valueString
	}

	// get all native api resources
	nativeApiResourcesMap, err := getNativeApiResourcesMap(request.LeafClientSet, request.LeafDynamicClient)
	if err != nil {
		return fmt.Errorf("get native api resource failed, err: %s", err)
	}

	// get all crds in leaf
	leafCRDList, err := listCRD(request.LeafDynamicClient)
	if err != nil {
		return fmt.Errorf("leaf client get crd failed, err: %s", err)
	}
	leafCRDMap, err := utils.ToMapSetE(leafCRDList)
	if err != nil {
		return fmt.Errorf("includedResources convert to map failed, err: %s", err)
	}
	leafCRDStringMap := make(map[string]*apiextensionsv1.CustomResourceDefinition)
	for _, value := range leafCRDMap.(map[interface{}]interface{}) {
		crd := value.(*apiextensionsv1.CustomResourceDefinition)
		leafCRDStringMap[crd.Name] = crd
	}

	// get all crds in root
	rootCRDList, err := listCRD(request.RootDynamicClient)
	if err != nil {
		return fmt.Errorf("root client get crd failed, err: %s", err)
	}
	rootCRDMap, err := utils.ToMapSetE(rootCRDList)
	if err != nil {
		return fmt.Errorf("includedResources convert to map failed, err: %s", err)
	}
	rootCRDStringMap := make(map[string]*apiextensionsv1.CustomResourceDefinition)
	for _, value := range rootCRDMap.(map[interface{}]interface{}) {
		crd := value.(*apiextensionsv1.CustomResourceDefinition)
		rootCRDStringMap[crd.Name] = crd
	}

	// judge whether the preferred version of resources for root cluster and leaf cluster is the same
	for _, indcludeResource := range includedResources {
		// not judge excluded resource
		if _, ok := excludeStringMap[indcludeResource]; ok {
			continue
		}
		// not judge native api resource
		if _, ok := nativeApiResourcesMap[indcludeResource]; ok {
			continue
		}

		leafCRD, ok := leafCRDStringMap[indcludeResource]
		if ok {
			return fmt.Errorf("crd %s do not exist in the leaf cluster", leafCRD.Name)
		}
		rootCRD, ok := rootCRDStringMap[indcludeResource]
		if ok {
			return fmt.Errorf("crd %s do not exist in the root cluster", rootCRD.Name)
		}
		if !strings.EqualFold(leafCRD.Spec.Versions[0].Name, rootCRD.Spec.Versions[0].Name) {
			return fmt.Errorf("crd %s version is different in that it is %s in leaf cluster and %s in root cluster",
				rootCRD.Name, leafCRD.Spec.Versions[0].Name, rootCRD.Spec.Versions[0].Name)
		}
	}

	return nil
}

func getNativeApiResourcesMap(clientSet kubernetes.Interface, dynamicClient dynamic.Interface) (map[string]string, error) {
	nativeApiResourcesMap, err := getApiResourcesMap(clientSet)
	if err != nil {
		return nil, fmt.Errorf("precheck failed, getApiResourcesMap in leaf cluster fauled, err: %s", err)
	}

	leafCRDList, err := listCRD(dynamicClient)
	if err != nil {
		return nil, fmt.Errorf("leaf client get crd failed, err: %s", err)
	}
	for _, crd := range leafCRDList {
		delete(nativeApiResourcesMap, crd.Name)
	}
	return nativeApiResourcesMap, nil
}

// getApiResourcesMap gets all preferred api resources for cluster
func getApiResourcesMap(clientSet kubernetes.Interface) (map[string]string, error) {
	apiResources, err := clientSet.Discovery().ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("get api-reources in leaf failed, err: %s", err)
	}
	apiResourcesMap, err := utils.ToMapSetE(apiResources)
	if err != nil {
		return nil, fmt.Errorf("apiResources convert to map failed, err: %s", err)
	}
	apiResourcesStringMap := make(map[string]string)
	for _, value := range apiResourcesMap.(map[interface{}]interface{}) {
		valueString := value.(*metav1.APIResourceList)
		groupVersion := valueString.GroupVersion
		var group, version string
		if i := strings.Index(valueString.GroupVersion, "/"); i >= 0 {
			group = groupVersion[:i]
			version = groupVersion[i+1:]
		} else {
			group = ""
			version = groupVersion
		}
		for _, resource := range valueString.APIResources {
			nameGroup := resource.Name
			if group != "" {
				nameGroup = fmt.Sprintf("%s.%s", nameGroup, group)
			}
			apiResourcesStringMap[nameGroup] = version
		}
	}
	return apiResourcesStringMap, nil
}

func checkNamespaces(request *requests.PromoteRequest, forbidNamespace []string) error {
	includes := request.NamespaceIncludesExcludes.GetIncludes()
	excludes := request.NamespaceIncludesExcludes.GetExcludes()
	leafNamespaceList, err := request.LeafClientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	rootNamespaceList, err := request.RootClientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// it is meaningless to include * in exclude
	for _, exclude := range excludes {
		if exclude == "*" {
			return fmt.Errorf("precheck failed, excludes has \"*\" ")
		}
	}

	for _, include := range includes {
		// add all resources to includes
		if include == "*" {
			var tmp []string
			for _, item := range leafNamespaceList.Items {
				tmp = append(tmp, item.Name)
			}
			includes = tmp
			break
		}
	}

	// needsStringMap removes namespace from exclude
	needsMap, err := utils.ToMapSetE(includes)
	if err != nil {
		return fmt.Errorf("includes convert to map failed, err: %s", err)
	}
	needsStringMap := make(map[string]string)
	for _, value := range needsMap.(map[interface{}]interface{}) {
		valueString := value.(string)
		needsStringMap[valueString] = valueString
	}

	for _, exclude := range excludes {
		value, found := needsStringMap[exclude]
		if !found {
			return fmt.Errorf("excludes has wrong namespace: %s", value)
		}
		delete(needsStringMap, exclude)
	}

	for _, forbid := range forbidNamespace {
		if _, ok := needsStringMap[forbid]; ok {
			return fmt.Errorf("promote this %s namesapcethe is forbidden", forbid)
		}
	}

	// judge whether the leaf cluster contains the namespace
	leafNamespaceMap, err := utils.ToMapSetE(leafNamespaceList.Items)
	if err != nil {
		return fmt.Errorf("leafNamespaceList convert to map failed, err: %s", err)
	}
	leafNamespaceStringMap := make(map[string]corev1.Namespace)
	for _, value := range leafNamespaceMap.(map[interface{}]interface{}) {
		namespace := value.(corev1.Namespace)
		leafNamespaceStringMap[namespace.Name] = namespace
	}

	for _, need := range needsStringMap {
		if _, ok := leafNamespaceStringMap[need]; !ok {
			return fmt.Errorf("precheck failed, leaf cluster don't have this namespace: %s", need)
		}
	}

	// judge whether the master cluster already contains the namespace in include
	rootNamespaceMap, err := utils.ToMapSetE(rootNamespaceList.Items)
	if err != nil {
		return fmt.Errorf("rootNamespaceList convert to map failed, err: %s", err)
	}
	rootNamespaceStringMap := make(map[string]corev1.Namespace)
	for _, value := range rootNamespaceMap.(map[interface{}]interface{}) {
		namespace := value.(corev1.Namespace)
		rootNamespaceStringMap[namespace.Name] = namespace
	}
	for _, need := range needsStringMap {
		if _, ok := rootNamespaceStringMap[need]; ok {
			return fmt.Errorf("precheck failed, the same namespace exists for the master cluster and leaf cluster: %s", need)
		}
	}
	return nil
}

// listCRD retrieves the list of crds from Kubernetes.
func listCRD(dynamicClient dynamic.Interface) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	objs, err := dynamicClient.Resource(constants.GVR_CRD).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	retObj := make([]*apiextensionsv1.CustomResourceDefinition, 0)

	for _, obj := range objs.Items {
		tmpObj := &apiextensionsv1.CustomResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &tmpObj); err != nil {
			return nil, err
		}
		retObj = append(retObj, tmpObj)
	}

	return retObj, nil
}
