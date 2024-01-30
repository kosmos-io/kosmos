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

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/backup"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/constants"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/detach"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/restore"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/collections"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

const (
	PromotePolicyControllerName = "promote-policy-controller"
	RequeueTime                 = 10 * time.Second
)

type PromotePolicyController struct {
	RootClient          client.Client
	RootClientSet       kubernetes.Interface
	RootDynamicClient   *dynamic.DynamicClient
	RootDiscoveryClient *discovery.DiscoveryClient
	GlobalLeafManager   leafUtils.LeafResourceManager
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
			klog.Infof("syncleaf %s not found", original.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("error getting syncleaf %s: %v", original.Name, err)
		return ctrl.Result{}, nil
	}

	switch original.Status.Phase {
	case "":
	// only process new backups
	default:
		klog.Infof("syncleaf %s is not handled", original.Name)
		return ctrl.Result{}, nil
	}

	lr, err := p.GlobalLeafManager.GetLeafResourceByNodeName(original.Spec.ClusterName)
	if err != nil {
		// wait for leaf resource init
		klog.Errorf("Error get leaf %s resource. %v", original.Spec.ClusterName, err)
		return reconcile.Result{RequeueAfter: RequeueTime}, nil
	}

	promoteRequest, err := p.preparePromoteRequest(original, lr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error prepareSyncRequest: %v", err)
	}

	backupFile, err := runBackup(promoteRequest)
	if err != nil {
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedBackup
		promoteRequest.Status.FailureReason = err.Error()
		if err := p.RootClient.Patch(context.TODO(), promoteRequest.PromotePolicy, client.MergeFrom(original)); err != nil {
			klog.Errorf("error updating syncleaf %s final status", original.Name)
		}
		return reconcile.Result{}, err
	}

	err = runDetach(promoteRequest, backupFile)
	if err != nil {
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedDetach
		promoteRequest.Status.FailureReason = err.Error()
		err := p.RootClient.Patch(context.TODO(), promoteRequest.PromotePolicy, client.MergeFrom(original))
		if err != nil {
			klog.Errorf("error updating syncleaf %s final status", original.Name)
		}
		return reconcile.Result{}, err
	}

	err = runRestore(promoteRequest, backupFile)
	if err != nil {
		promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseFailedRestore
		promoteRequest.Status.FailureReason = err.Error()
		err := p.RootClient.Patch(context.TODO(), promoteRequest.PromotePolicy, client.MergeFrom(original))
		if err != nil {
			klog.Errorf("error updating syncleaf %s final status", original.Name)
		}
		return reconcile.Result{}, err
	}

	promoteRequest.Status.Phase = v1alpha1.PromotePolicyPhaseCompleted
	err = p.RootClient.Patch(context.TODO(), promoteRequest.PromotePolicy, client.MergeFrom(original))
	if err != nil {
		klog.Errorf("error updating syncleaf %s final status", original.Name)
	}

	return reconcile.Result{}, nil
}

func (s *PromotePolicyController) preparePromoteRequest(promote *v1alpha1.PromotePolicy, lf *leafUtils.LeafResource) (*requests.PromoteRequest, error) {
	// todo validate params

	request := &requests.PromoteRequest{
		PromotePolicy:             promote.DeepCopy(),
		RootClientSet:             s.RootClientSet,
		RootDynamicClient:         s.RootDynamicClient,
		RootDiscoveryClient:       s.RootDiscoveryClient,
		LeafClientSet:             lf.Clientset,
		LeafDynamicClient:         lf.DynamicClient,
		LeafDiscoveryClient:       lf.DiscoveryClient,
		NamespaceIncludesExcludes: collections.NewIncludesExcludes().Includes(promote.Spec.IncludedNamespaces...).Excludes(promote.Spec.ExcludedNamespaces...),
		BackedUpItems:             make(map[requests.ItemKey]struct{}),
		DetachedItems:             make(map[requests.ItemKey]struct{}),
		RestoredItems:             make(map[requests.ItemKey]struct{}),
	}
	return request, nil
}

func runBackup(promoteRequest *requests.PromoteRequest) (file string, err error) {
	klog.Info("Setting up backup temp file")
	filePath := constants.BackupDir + time.Now().Format("20060102-150405")
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

func runRestore(promoteRequest *requests.PromoteRequest, backupfile string) error {
	// 打开压缩文件
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
