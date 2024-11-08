package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PodSyncControllerName = "pod-sync-controller"

	DefaultpodStatusUpdateInterval = 10 * time.Second
)

type PodSyncController struct {
	leafClient kubernetes.Interface
	root       client.Client

	podstatusInterval time.Duration
}

func NewPodSyncController(leafClient kubernetes.Interface, root client.Client) *PodSyncController {
	c := &PodSyncController{
		leafClient:        leafClient,
		root:              root,
		podstatusInterval: DefaultpodStatusUpdateInterval,
	}
	return c
}

func (c *PodSyncController) Start(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.syncPodStatus, c.podstatusInterval)
	<-ctx.Done()
	return nil
}

func (c *PodSyncController) syncPodStatus(ctx context.Context) {
	err := c.updatePodStatus(ctx)
	if err != nil {
		klog.Errorf(err.Error())
	}
}

func (c *PodSyncController) updatePodStatus(ctx context.Context) error {
	Selector := labels.SelectorFromSet(
		map[string]string{
			utils.KosmosPodLabel: "true",
		})
	pods, err := c.leafClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: Selector.String(),
	})
	if err != nil {
		klog.Errorf("Could not list pods in leaf cluster,Error: %v", err)
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(pods.Items))

	for _, leafpod := range pods.Items {
		wg.Add(1)
		go func(leafpod corev1.Pod) {
			defer wg.Done()
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				rootpod := &corev1.Pod{}
				if err := c.root.Get(ctx, types.NamespacedName{Name: leafpod.Name, Namespace: leafpod.Namespace}, rootpod); err != nil {
					if apierrors.IsNotFound(err) {
						klog.Warningf("Pod %s in namespace %s not found in root cluster, skipping...", leafpod.Name, leafpod.Namespace)
						return nil
					}
					return err
				}
				if podutils.IsKosmosPod(rootpod) && !reflect.DeepEqual(rootpod.Status, leafpod.Status) {
					rPodCopy := rootpod.DeepCopy()
					rPodCopy.Status = leafpod.Status
					podutils.FitObjectMeta(&rPodCopy.ObjectMeta)
					if err := c.root.Status().Update(ctx, rPodCopy); err != nil && !apierrors.IsNotFound(err) {
						klog.V(4).Info(errors.Wrap(err, "error while updating pod status in kubernetes"))
						return err
					}
				}
				return nil
			})
			if err != nil {
				//klog.Errorf("failed to update pod %s/%s, error: %v", leafpod.Namespace, leafpod.Name, err)
				errChan <- fmt.Errorf("failed to update pod %s/%s, error: %v", leafpod.Namespace, leafpod.Name, err)
			}
		}(leafpod)
	}

	wg.Wait()
	close(errChan)

	var taskErr error
	for err := range errChan {

		if taskErr == nil {
			taskErr = err
		} else {
			taskErr = errors.Wrap(err, taskErr.Error())
		}
	}

	if taskErr != nil {
		return taskErr
	}

	return nil
}
