//nolint:revive
package clusterlink_operator

import (
	"context"
	"os"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cmdOptions "github.com/kosmos.io/kosmos/cmd/clusterlink/clusterlink-operator/app/options"
	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/option"
	lister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	controllerName             = "operator-controller"
	ClusterControllerFinalizer = "kosmos.io/operator-controller"
)

// nolint
type Reconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	ClusterLister          lister.ClusterLister
	ControlPanelKubeConfig *clientcmdapi.Config
	ClusterName            string
	Options                *cmdOptions.Options
}

// SetupWithManager is for controller register
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.Cluster{}).
		Complete(r)
}

func (r *Reconciler) syncCluster(cluster *v1alpha1.Cluster) (reconcile.Result, error) {
	klog.Infof("install agent")
	useProxy := r.Options.UseProxy
	if value, exist := os.LookupEnv(utils.EnvUseProxy); exist {
		boo, err := strconv.ParseBool(value)
		if err != nil {
			klog.Warningf("parse env %s to bool err: %v", utils.EnvUseProxy, err)
		}
		useProxy = boo
	}
	opt := &option.AddonOption{
		Cluster:                *cluster,
		KubeConfigByte:         cluster.Spec.Kubeconfig,
		ControlPanelKubeConfig: r.ControlPanelKubeConfig,
		UseProxy:               useProxy,
	}

	if err := opt.Complete(); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}

	if err := Install(opt); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}
	return r.ensureFinalizer(cluster)
}

func (r *Reconciler) removeCluster(cluster *v1alpha1.Cluster) (reconcile.Result, error) {
	klog.Infof("uninstall agent")
	opt := &option.AddonOption{
		Cluster:                *cluster,
		KubeConfigByte:         cluster.Spec.Kubeconfig,
		ControlPanelKubeConfig: r.ControlPanelKubeConfig,
	}
	if err := opt.Complete(); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}
	if err := Uninstall(opt); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}

	return r.removeFinalizer(cluster)
}

func (r *Reconciler) ensureFinalizer(cluster *v1alpha1.Cluster) (reconcile.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, ClusterControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	controllerutil.AddFinalizer(cluster, ClusterControllerFinalizer)
	err := r.Client.Update(context.TODO(), cluster)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) removeFinalizer(cluster *v1alpha1.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, ClusterControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	controllerutil.RemoveFinalizer(cluster, ClusterControllerFinalizer)
	err := r.Client.Update(context.TODO(), cluster)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

// Reconcile is for controller reconcile
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Reconciling cluster %s", request.NamespacedName.Name)

	//if request.NamespacedName.Name != r.ClusterName {
	//	klog.Infof("Skip this event")
	//	return reconcile.Result{}, nil
	//}

	cluster := &v1alpha1.Cluster{}
	if err := r.Client.Get(ctx, request.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{Requeue: true}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		if len(cluster.GetFinalizers()) == 1 {
			return r.removeCluster(cluster)
		}
	}

	if !cluster.Spec.ClusterLinkOptions.Enable {
		klog.Infof("cluster %v does not have the clusterlink module enabled, skipping this event.", cluster.Name)
		return reconcile.Result{}, nil
	}

	return r.syncCluster(cluster)
}
