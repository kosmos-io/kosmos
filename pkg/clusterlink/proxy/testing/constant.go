package testing

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

var (
	PodGVK     = corev1.SchemeGroupVersion.WithKind("Pod")
	SecretGVR  = corev1.SchemeGroupVersion.WithKind("Secret")
	RestMapper *meta.DefaultRESTMapper

	PodGVR = corev1.SchemeGroupVersion.WithResource("pods")

	PodSelectorWithNS1 = v1alpha1.ResourceCacheSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind, Namespace: []string{"ns1"}}

	PodSelectorWithNS2 = v1alpha1.ResourceCacheSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind, Namespace: []string{"ns2"}}

	PodResourceCacheSelector = v1alpha1.ResourceCacheSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind, Namespace: []string{"ns1", "ns2"}}
)

func init() {
	RestMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	RestMapper.Add(PodGVK, meta.RESTScopeNamespace)
	RestMapper.Add(SecretGVR, meta.RESTScopeNamespace)
}
