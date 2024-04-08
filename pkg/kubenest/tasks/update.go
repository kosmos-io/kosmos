package tasks

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewUpdateVirtualClusterObjectTask() workflow.Task {
	return workflow.Task{
		Name:        "update-virtual-cluster-object",
		Run:         runUpdateVirtualClusterObject,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "update-kubeconfig",
				Run:  runUpdateKubeConfig,
			},
			{
				Name: "update-status",
				Run:  runUpdateStatus,
			},
		},
	}
}

func runUpdateVirtualClusterObject(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return fmt.Errorf("update-virtual-cluster-object task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[update-virtual-cluster-object] Running task", "virtual cluster", klog.KObj(data))
	return nil
}

func runUpdateKubeConfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("update-kubeconfig task invoked with an invalid data struct")
	}

	reconcileVirtualCluster, err := data.KosmosClient().KosmosV1alpha1().VirtualClusters().Get(context.Background(), data.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.New("update-kubeconfig task : can not get virtual cluster object")
		}
	}
	reconcileVirtualCluster.Spec.Kubeconfig = base64.StdEncoding.EncodeToString(data.ControlplaneConfig().TLSClientConfig.CertData)
	_, err = data.KosmosClient().KosmosV1alpha1().VirtualClusters().Update(context.TODO(), reconcileVirtualCluster, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update kubeconfig for virtualCluster Object, err: %w", err)
	}

	klog.V(2).InfoS("[update-kubeconfig] Successfully updated kubeconfig for virtualCluster Object", "virtual cluster", klog.KObj(data))
	return nil
}

func runUpdateStatus(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("update-status task invoked with an invalid data struct")
	}

	reconcileVirtualCluster, err := data.KosmosClient().KosmosV1alpha1().VirtualClusters().Get(context.Background(), data.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.New("update-status task : can not get virtual cluster object")
		}
	}
	reconcileVirtualCluster.Status.Phase = v1alpha1.Completed
	_, err = data.KosmosClient().KosmosV1alpha1().VirtualClusters().Update(context.TODO(), reconcileVirtualCluster, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update status for virtualCluster Object, err: %w", err)
	}

	klog.V(2).InfoS("[update-status] Successfully updated status for virtualCluster Object", "virtual cluster", klog.KObj(data))
	return nil
}
