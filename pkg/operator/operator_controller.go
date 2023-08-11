package operator

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

	cmdOptions "github.com/kosmos.io/clusterlink/cmd/operator/app/options"
	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	clusterlinkv1alpha1lister "github.com/kosmos.io/clusterlink/pkg/generated/listers/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/option"
	"github.com/kosmos.io/clusterlink/pkg/utils"
)

const (
	controllerName             = "operator-controller"
	ClusterControllerFinalizer = "cnp.io/operator-controller"
)

// nolint
type Reconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	ClusterLister          clusterlinkv1alpha1lister.ClusterLister
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
		For(&clusterlinkv1alpha1.Cluster{}).
		Complete(r)
}

func (r *Reconciler) syncCluster(cluster *clusterlinkv1alpha1.Cluster) (reconcile.Result, error) {
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
		ControlPanelKubeConfig: r.ControlPanelKubeConfig,
		UseProxy:               useProxy,
	}

	if err := opt.Complete(r.Options); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}

	if err := addons.Install(opt); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}
	return r.ensureFinalizer(cluster)
}

func (r *Reconciler) removeCluster(cluster *clusterlinkv1alpha1.Cluster) (reconcile.Result, error) {
	klog.Infof("uninstall agent")
	opt := &option.AddonOption{
		Cluster:                *cluster,
		ControlPanelKubeConfig: r.ControlPanelKubeConfig,
	}
	if err := opt.Complete(r.Options); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}
	if err := addons.Uninstall(opt); err != nil {
		klog.Error(err)
		return reconcile.Result{Requeue: true}, err
	}

	return r.removeFinalizer(cluster)
}

func (r *Reconciler) ensureFinalizer(cluster *clusterlinkv1alpha1.Cluster) (reconcile.Result, error) {
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

func (r *Reconciler) removeFinalizer(cluster *clusterlinkv1alpha1.Cluster) (reconcile.Result, error) {
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
	klog.Infof("Reconciling cluster %s, operator cluster name %s", request.NamespacedName.Name, r.ClusterName)

	if request.NamespacedName.Name != r.ClusterName {
		klog.Infof("Skip this event")
		return reconcile.Result{}, nil
	}

	cluster := &clusterlinkv1alpha1.Cluster{}

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

	return r.syncCluster(cluster)
}
