package rootpodsyncers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type ServerlessSyncer struct {
	RootClient kubernetes.Interface
}

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

func (r *ServerlessSyncer) DeletePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName, cleanflag bool) error {
	// TODO: poolid set in leafresource
	if err := lr.ServerlessClient.DeletePod(rootnamespacedname.Name); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if cleanflag {
		return DeletePodInRootCluster(ctx, rootnamespacedname, r.RootClient)
	}
	return nil
}

func (r *ServerlessSyncer) CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) error {
	if pod.Labels != nil {
		if _, ok := pod.Labels[utils.LabelServerlessOrderIds]; ok {
			return nil
		}
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[utils.KosmosPodLabel] = "true"

	podCopy := pod.DeepCopy()

	if _, err := lr.ServerlessClient.GetPods(pod.Name, ""); err != nil {
		if errors.IsNotFound(err) {
			if orderInfo, err := lr.ServerlessClient.CreatePod(pod.Name, podCopy); err != nil {
				return err
			} else {
				// update pod label/ do not recreate
				pod.Labels[utils.LabelServerlessOrderIds] = orderInfo.OrderId
				if _, err := r.RootClient.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
					// TODO: recreate
					return fmt.Errorf("cannot update pod lable orderids, err: %s", err)
				}
			}
		} else {
			return err
		}
	}
	// has created
	return nil
}

func (r *ServerlessSyncer) UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootpod *corev1.Pod, leafpod *corev1.Pod) error {
	// No update openapi
	return fmt.Errorf("not implemented")
}

func (r *ServerlessSyncer) GetPodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName) (*corev1.Pod, error) {
	// TODO: poolid set in leafresource
	pod, err := r.RootClient.CoreV1().Pods(rootnamespacedname.Namespace).Get(ctx, rootnamespacedname.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if pod.Labels == nil {
		return nil, errors.NewNotFound(corev1.Resource("pod"), rootnamespacedname.Name)
	}
	if _, ok := pod.Labels[utils.LabelServerlessOrderIds]; !ok {
		return nil, errors.NewNotFound(corev1.Resource("pod"), rootnamespacedname.Name)
	}
	return lr.ServerlessClient.GetPods(rootnamespacedname.Name, pod.Labels[utils.LabelServerlessOrderIds])
}
