package promote

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/backup"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/constants"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/detach"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/precheck"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/restore"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/types"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/collections"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

const (
	PromotePolicyControllerName = "promote-policy-controller"
	RequeueTime                 = 10 * time.Second
)

type PromotePolicyController struct {
	RootClient           client.Client
	RootClientSet        kubernetes.Interface
	RootDynamicClient    *dynamic.DynamicClient
	RootDiscoveryClient  *discovery.DiscoveryClient
	GlobalLeafManager    leafUtils.LeafResourceManager
	PromotePolicyOptions options.PromotePolicyOptions
}

func (p *PromotePolicyController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(PromotePolicyControllerName).
		For(&v1alpha1.PromotePolicy{}).
		Complete(p)
}

func (p *PromotePolicyController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	original := &v1alpha1.PromotePolicy{}
	if err := p.RootClient.Get(ctx, request.NamespacedName, original); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("promotepolicy %s not found", original.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("error getting promotepolicy %s: %v", original.Name, err)
		return ctrl.Result{}, nil
	}

	lr, err := p.GlobalLeafManager.GetLeafResourceByNodeName("kosmos-" + original.Spec.ClusterName)
	if err != nil {
		// wait for leaf resource init
		klog.Errorf("Error get kosmos leaf %s resource. %v", original.Spec.ClusterName, err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	promoteRequest, err := p.preparePromoteRequest(original, lr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error prepare promoteRequest: %v", err)
	}

	switch original.Status.Phase {
	case "":
	//create promotepolicy request
	case v1alpha1.PromotePolicyPhaseCompleted, v1alpha1.PromotePolicyPhaseFailedRollback:
		// check if Rollback request
		if original.Spec.Rollback == "true" {
			klog.Info("rollback start...")
			promoteRequest.Spec.Rollback = ""
			err = DetachRollback(promoteRequest, original.Status.BackedupFile, true)
			if err != nil {
				klog.Errorf("rollback detached resources err: %s", err.Error())
				promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedRollback
				promoteRequest.Status.FailureReason = err.Error()
				if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
					return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
				}
				return ctrl.Result{}, nil
			}

			err = RestoreRollback(promoteRequest, original.Status.BackedupFile, true)
			if err != nil {
				klog.Errorf("rollback restored resources err: %s", err.Error())
				promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedRollback
				promoteRequest.Status.FailureReason = err.Error()
				if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
					return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
				}
				return ctrl.Result{}, nil
			}

			promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseRolledback
			if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
			}
		}

		return ctrl.Result{}, nil
	default:
		klog.Infof("promotePolicy %s status %s will not handled", original.Name, original.Status.Phase)
		return ctrl.Result{}, nil
	}

	err = runPrecheck(promoteRequest)
	if err != nil {
		klog.Errorf("precheck err: %s", err.Error())
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedPrecheck
		promoteRequest.Status.FailureReason = err.Error()
		if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
		}
		return reconcile.Result{}, err
	}

	backupFile, err := runBackup(promoteRequest)
	if err != nil {
		klog.Errorf("backup resources err: %s", err.Error())
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedBackup
		promoteRequest.Status.FailureReason = err.Error()
		if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
		}
		return reconcile.Result{}, err
	}
	klog.Infof("backup success. file: %s", backupFile)

	promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseDetach
	promoteRequest.Status.BackedupFile = backupFile
	if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
	}

	err = runDetach(promoteRequest, backupFile)
	if err != nil {
		klog.Errorf("detach resources err: %s", err.Error())
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedDetach
		promoteRequest.Status.FailureReason = err.Error()
		if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
		}

		klog.Warning("Begin rollback detached resources because detach stage failed.")
		time.Sleep(5 * time.Second)
		err = DetachRollback(promoteRequest, backupFile, false)
		if err != nil {
			klog.Errorf("rollback detached resource err: %s", err.Error())
		} else {
			klog.Info("all detached resource rollback suceess.")
		}
		return reconcile.Result{}, err
	}

	err = runRestore(promoteRequest, backupFile)
	if err != nil {
		klog.Errorf("restore resources err: %s", err.Error())
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedRestore
		promoteRequest.Status.FailureReason = err.Error()
		if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
		}

		klog.Warning("Begin rollback detached and restored resources because restore stage failed.")
		time.Sleep(5 * time.Second)
		err = DetachRollback(promoteRequest, backupFile, true)
		if err != nil {
			klog.Errorf("rollback detached resource err: %s", err.Error())
		} else {
			klog.Info("all detached resource rollback suceess.")
		}

		err = RestoreRollback(promoteRequest, backupFile, false)
		if err != nil {
			klog.Errorf("rollback restored resource err: %s", err.Error())
		} else {
			klog.Info("all restored resource rollback suceess.")
		}
		return reconcile.Result{}, err
	}

	promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseCompleted
	if err = p.updateStatus(original, promoteRequest.PromotePolicy); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error updating promotepolicy %s to status %s", original.Name, promoteRequest.Status.Phase)
	}

	klog.Infof("Create promotePolicy %s completed", original.Name)

	return reconcile.Result{}, nil
}

func (p *PromotePolicyController) updateStatus(original *v1alpha1.PromotePolicy, updatedObj *v1alpha1.PromotePolicy) error {
	return p.RootClient.Patch(context.TODO(), updatedObj, client.MergeFrom(original))
}

func (p *PromotePolicyController) preparePromoteRequest(promote *v1alpha1.PromotePolicy, lf *leafUtils.LeafResource) (*requests.PromoteRequest, error) {
	// todo validate params

	request := &requests.PromoteRequest{
		PromotePolicy:             promote.DeepCopy(),
		RootClientSet:             p.RootClientSet,
		RootDynamicClient:         p.RootDynamicClient,
		RootDiscoveryClient:       p.RootDiscoveryClient,
		LeafClientSet:             lf.Clientset,
		LeafDynamicClient:         lf.DynamicClient,
		LeafDiscoveryClient:       lf.DiscoveryClient,
		NamespaceIncludesExcludes: collections.NewIncludesExcludes().Includes(promote.Spec.IncludedNamespaces...).Excludes(promote.Spec.ExcludedNamespaces...),
		BackedUpItems:             make(map[types.ItemKey]struct{}),
		DetachedItems:             make(map[types.ItemKey]struct{}),
		RestoredItems:             make(map[types.ItemKey]types.RestoredItemStatus),
		ForbidNamespaces:          p.PromotePolicyOptions.ForbidNamespaces,
	}
	return request, nil
}

func runPrecheck(promoteRequest *requests.PromoteRequest) error {
	klog.Info("start precheck...")
	prechecker, err := precheck.NewKubernetesPrecheck(promoteRequest)
	if err != nil {
		return errors.Wrap(err, "error new precheck instance")
	}

	err = prechecker.Precheck()
	if err != nil {
		return errors.Wrap(err, "error precheck")
	}

	return nil
}

func runBackup(promoteRequest *requests.PromoteRequest) (file string, err error) {
	klog.Info("start backup resources")
	filePath := constants.BackupDir + promoteRequest.Name + time.Now().Format("20060102-150405")
	backupFile, err := os.Create(filePath)
	if err != nil {
		return "", errors.Wrap(err, "error creating temp file for backup")
	}
	defer backupFile.Close()

	backuper, err := backup.NewKubernetesBackupper(promoteRequest)
	if err != nil {
		return "", errors.Wrap(err, "error new backup instance")
	}

	err = backuper.Backup(backupFile)
	if err != nil {
		return "", errors.Wrap(err, "error backup")
	}

	return filePath, nil
}

func runDetach(promoteRequest *requests.PromoteRequest, backupfile string) error {
	// 打开压缩文件
	backupReader, err := os.Open(backupfile)
	if err != nil {
		panic(err)
	}
	defer backupReader.Close()

	detacher, err := detach.NewKubernetesDetacher(promoteRequest, backupReader)
	if err != nil {
		return errors.Wrap(err, "error new detach instance")
	}

	err = detacher.Detach()
	if err != nil {
		return errors.Wrap(err, "error detach")
	}

	return nil
}

func DetachRollback(promoteRequest *requests.PromoteRequest, backupfile string, detachSuccess bool) error {
	backupReader, err := os.Open(backupfile)
	if err != nil {
		panic(err)
	}
	defer backupReader.Close()

	detacher, err := detach.NewKubernetesDetacher(promoteRequest, backupReader)
	if err != nil {
		return errors.Wrap(err, "error new detach instance")
	}

	err = detacher.Rollback(detachSuccess)
	if err != nil {
		return errors.Wrap(err, "error detach")
	}
	return nil
}

func RestoreRollback(promoteRequest *requests.PromoteRequest, backupfile string, restoreSuccess bool) error {
	backupReader, err := os.Open(backupfile)
	if err != nil {
		panic(err)
	}
	defer backupReader.Close()

	restorer, err := restore.NewKubernetesRestorer(promoteRequest, backupReader)
	if err != nil {
		return errors.Wrap(err, "error new restore instance")
	}
	err = restorer.Rollback(restoreSuccess)
	if err != nil {
		return errors.Wrap(err, "error restore")
	}
	return nil
}

func runRestore(promoteRequest *requests.PromoteRequest, backupfile string) error {
	backupReader, err := os.Open(backupfile)
	if err != nil {
		panic(err)
	}
	defer backupReader.Close()

	restorer, err := restore.NewKubernetesRestorer(promoteRequest, backupReader)
	if err != nil {
		return errors.Wrap(err, "error new restore instance")
	}
	err = restorer.Restore()
	if err != nil {
		return errors.Wrap(err, "error restore")
	}

	return nil
}
