package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

var (
	etcdLabels = labels.Set{constants.Label: constants.Etcd}
)

func NewEtcdTask() workflow.Task {
	return workflow.Task{
		Name:        "Etcd",
		Run:         runEtcd,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-etcd",
				Run:  runDeployEtcd,
			},
			{
				Name: "check-etcd",
				Run:  runCheckEtcd,
			},
		},
	}
}

func runEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("etcd task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[etcd] Running etcd task", "virtual cluster", klog.KObj(data))
	return nil
}

func runDeployEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("deploy-etcd task invoked with an invalid data struct")
	}

	err := controlplane.EnsureVirtualClusterEtcd(data.RemoteClient(), data.GetName(), data.GetNamespace(), data.KubeNestOpt(), data.VirtualCluster())
	if err != nil {
		return fmt.Errorf("failed to install etcd component, err: %w", err)
	}

	klog.V(2).InfoS("[deploy-etcd] Successfully installed etcd component", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-etcd task invoked with an invalid data struct")
	}

	checker := apiclient.NewVirtualClusterChecker(data.RemoteClient(), constants.ComponentBeReadyTimeout)

	if err := checker.WaitForSomePods(etcdLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("checking for virtual cluster etcd to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[check-etcd] the etcd pods is ready", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallEtcdTask() workflow.Task {
	return workflow.Task{
		Name:        "Etcd",
		Run:         runEtcd,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.Etcd,
				Run:  UninstallEtcd,
			},
		},
	}
}

func DeleteEtcdPvcTask() workflow.Task {
	return workflow.Task{
		Name:        "Etcd",
		Run:         runEtcd,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.Etcd,
				Run:  deleteEtcdPvc,
			},
			{
				Name: "check-pvc-deleted",
				Run:  checkPvcDeleted,
			},
		},
	}
}

func UninstallEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("destroy-etcd task invoked with an invalid data struct")
	}

	err := controlplane.DeleteVirtualClusterEtcd(data.RemoteClient(), data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to uninstall etcd component, err: %w", err)
	}

	klog.V(2).InfoS("[uninstall-etcd] Successfully uninstalled etcd component", "virtual cluster", klog.KObj(data))
	return nil
}

func deleteEtcdPvc(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("destroy-etcd task invoked with an invalid data struct")
	}

	for i := 0; i < constants.EtcdReplicas; i++ {
		pvc := fmt.Sprintf("%s-%s-etcd-%d", constants.EtcdDataVolumeName, data.GetName(), i)
		klog.V(2).Infof("Delete pvc %s/%s", pvc, data.GetNamespace())
		err := data.RemoteClient().CoreV1().PersistentVolumeClaims(data.GetNamespace()).Delete(context.TODO(), pvc, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return errors.Wrapf(err, "Delete pvc %s error", pvc)
		}
	}
	return nil
}

func checkPvcDeleted(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("destroy-etcd task invoked with an invalid data struct")
	}

	klog.V(2).Infof("Check if %s etcd pvc deleted", data.GetName())
	err := wait.PollImmediate(5*time.Second, constants.ComponentBeDeletedTimeout, func() (done bool, err error) {
		pvcList, err := data.RemoteClient().CoreV1().PersistentVolumeClaims(data.GetNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: virtualClusterEtcdLabels.String()})
		if err != nil {
			return true, errors.Wrap(err, "List pods error")
		}
		if len(pvcList.Items) == 0 {
			return true, nil
		}
		klog.V(2).Infof("Waiting for pvc deleted. current exist num: %d", len(pvcList.Items))
		return false, nil
	})
	if err != nil {
		return errors.Wrapf(err, "Failed delete etcd pvc")
	}
	return nil
}
