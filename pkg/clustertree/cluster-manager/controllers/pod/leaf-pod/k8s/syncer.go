package k8s

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

type leafPodK8sSyncer struct {
	LeafClient *kubernetes.Clientset
	RootClient kubernetes.Interface
}

const (
	LeafPodControllerName = "leaf-pod-controller"
	LeafPodRequeueTime    = 10 * time.Second
)

func DeletePodInRootCluster(ctx context.Context, rootnamespacedname runtime.NamespacedName, rootClient kubernetes.Interface) error {
	rPod, err := rootClient.CoreV1().Pods(rootnamespacedname.Namespace).Get(ctx, rootnamespacedname.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	rPodCopy := rPod.DeepCopy()

	if err := rootClient.CoreV1().Pods(rPodCopy.Namespace).Delete(ctx, rPodCopy.Name, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64),
	}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (s *leafPodK8sSyncer) Reconcile(ctx context.Context, key runtime.NamespacedName) (runtime.Result, error) {
	pod, err := s.LeafClient.CoreV1().Pods(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// delete pod in root
			if err := DeletePodInRootCluster(ctx, key, s.RootClient); err != nil {
				return runtime.Result{RequeueAfter: LeafPodRequeueTime}, nil
			}
			return runtime.Result{}, nil
		}

		klog.Errorf("get %s error: %v", key, err)
		return runtime.Result{RequeueAfter: LeafPodRequeueTime}, nil
	}

	podCopy := pod.DeepCopy()

	// if ShouldSkipStatusUpdate(podCopy) {
	// 	return reconcile.Result{}, nil
	// }

	if podutils.IsKosmosPod(podCopy) {
		podutils.FitObjectMeta(&podCopy.ObjectMeta)
		podCopy.ResourceVersion = "0"
		if _, err := s.RootClient.CoreV1().Pods(podCopy.Namespace).UpdateStatus(ctx, podCopy, metav1.UpdateOptions{}); err != nil && !errors.IsNotFound(err) {
			klog.V(4).Info(fmt.Sprintf("error while updating pod status in kubernetes: %s", err))
			return runtime.Result{RequeueAfter: LeafPodRequeueTime}, nil
		}
	}
	return runtime.Result{}, nil
}
