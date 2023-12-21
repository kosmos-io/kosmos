package podutils

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

var VolumePathGVR = schema.GroupVersionResource{
	Version:  os.Getenv("ONEWAY-VERSION"),
	Group:    os.Getenv("ONEWAY-GROUP"),
	Resource: os.Getenv("ONEWAY-RESOURCE"),
}

var (
	csiDriverName   = os.Getenv("ONEWAY-CSI-DRIVER-NAME")
	vpAnnotationKey = os.Getenv("ONEWAY-CSI-ANNOTATION-KEY")
)

func NoOneWayPVCFilter(ctx context.Context, dynamicRootClient dynamic.Interface, pvcNames []string, ns string) ([]string, error) {
	if len(os.Getenv("USE-ONEWAY-STORAGE")) == 0 {
		return pvcNames, nil
	}
	result := []string{}
	for _, pvcName := range pvcNames {
		f, err := IsOneWayPVCByName(ctx, dynamicRootClient, pvcName, ns)
		if err != nil {
			return nil, err
		}
		if !f {
			result = append(result, pvcName)
		}
	}
	return result, nil
}

func IsOneWayPVCByName(ctx context.Context, dynamicRootClient dynamic.Interface, pvcName string, ns string) (bool, error) {
	rootobj, err := dynamicRootClient.Resource(utils.GVR_PVC).Namespace(ns).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	pvcObj := &corev1.PersistentVolumeClaim{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(rootobj.Object, pvcObj)
	if err != nil {
		return false, err
	}

	return IsOneWayPVC(pvcObj), nil
}

func IsOneWayPVC(pvc *corev1.PersistentVolumeClaim) bool {
	if len(os.Getenv("USE-ONEWAY-STORAGE")) == 0 {
		return false
	}
	anno := pvc.GetAnnotations()
	if anno == nil {
		return false
	}
	if _, ok := anno[vpAnnotationKey]; ok {
		return true
	}
	return false
}

func IsOneWayPV(pv *corev1.PersistentVolume) bool {
	if len(os.Getenv("USE-ONEWAY-STORAGE")) == 0 {
		return false
	}
	return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == csiDriverName
}
