package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/cert"
	clusterManager "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	"github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	kosmosctl "github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/constants"
	treemanifest "github.com/kosmos.io/kosmos/pkg/treeoperator/manifest"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

type VirtualClusterJoinController struct {
	client.Client
	EventRecorder    record.EventRecorder
	KubeconfigPath   string
	KubeconfigStream []byte
}

func (c *VirtualClusterJoinController) RemoveClusterFinalizer(cluster *v1alpha1.Cluster, kosmosClient versioned.Interface) error {
	for _, finalizer := range []string{utils.ClusterStartControllerFinalizer, clusterManager.ControllerFinalizerName} {
		if controllerutil.ContainsFinalizer(cluster, finalizer) {
			controllerutil.RemoveFinalizer(cluster, finalizer)
		}
	}
	klog.Infof("remove finalizer for cluster %s", cluster.Name)

	_, err := kosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("cluster %s failed remove finalizer: %v", cluster.Name, err)
		return err
	}
	klog.Infof("update cluster after remove finalizer for cluster %s", cluster.Name)
	return nil
}

func (c *VirtualClusterJoinController) UninstallClusterTree(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Start deleting kosmos-clustertree deployment %s/%s-clustertree-cluster-manager...", request.Namespace, request.Name)
	clustertreeDeploy, err := kosmosctl.GenerateDeployment(treemanifest.ClusterTreeClusterManagerDeployment, treemanifest.DeploymentReplace{
		Namespace:       request.Namespace,
		ImageRepository: "null",
		Version:         "null",
		Name:            request.Name,
		FilePath:        treemanifest.DefaultKubeconfigPath,
	})
	if err != nil {
		return err
	}

	deleteRequest := types.NamespacedName{
		Namespace: request.Namespace,
		Name:      clustertreeDeploy.Name,
	}
	err = c.Get(ctx, deleteRequest, clustertreeDeploy)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get clustertree deployment %s-clustertree-cluster-manager  error, deployment deleted failed: %v",
				request.Name, err)
		}
		klog.Infof("clustertree deployment %s-clustertree-cluster-manager doesn't exist", request.Name)
	} else {
		err := c.Delete(ctx, clustertreeDeploy)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete kosmos-clustertree deployment %s-clustertree-cluster-manager  error: %v",
				request.Name, err)
		}
	}

	klog.Infof("Deployment %s/%s-clustertree-cluster-manager has been deleted. ", request.Namespace, request.Name)

	klog.Infof("Start deleting kosmos-clustertree secret %s/%s-clustertree-cluster-manager", request.Namespace, request.Name)
	clustertreeSecret, err := kosmosctl.GenerateSecret(treemanifest.ClusterTreeClusterManagerSecret, treemanifest.SecretReplace{
		Namespace: request.Namespace,
		Cert:      cert.GetCrtEncode(),
		Key:       cert.GetKeyEncode(),
		Name:      request.Name,
	})
	if err != nil {
		return err
	}
	deleteRequest.Name = clustertreeSecret.Name
	err = c.Get(ctx, deleteRequest, clustertreeSecret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get clustertree secret error, secret %s/%s-clustertree-cluster-manager deleted failed: %v",
				request.Namespace, request.Name, err)
		}
		klog.Infof("clustertree secret doesn't exist")
	} else {
		err := c.Delete(ctx, clustertreeSecret)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete kosmos-clustertree secret %s/%s-clustertree-cluster-manager error: %v", request.Namespace, request.Name, err)
		}
	}
	klog.Infof("Secret %s/%s-clustertree-cluster-manager has been deleted. ", request.Namespace, request.Name)

	clusterName := fmt.Sprintf("virtualcluster-%s-%s", request.Namespace, request.Name)
	klog.Infof("Attempting to delete cluster %s...", clusterName)

	kubeconfigStream, err := base64.StdEncoding.DecodeString(vc.Spec.Kubeconfig)
	if err != nil {
		return fmt.Errorf("decode target kubernetes kubeconfig %s err: %v", vc.Spec.Kubeconfig, err)
	}
	err, kosmosClient, _, _ := c.InitTargetKubeclient(kubeconfigStream)
	if err != nil {
		return fmt.Errorf("create kubernetes client failed: %v", err)
	}

	old, err := kosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(),
		clusterName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get cluster %s failed when we try to del: %v", clusterName, err)
		}
	} else {
		err = c.RemoveClusterFinalizer(old, kosmosClient)
		if err != nil {
			return fmt.Errorf("removefinalizer %s failed: %v", clusterName, err)
		}
		err = kosmosClient.KosmosV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("delete cluster %s failed: %v", clusterName, err)
		}
	}
	klog.Infof("Cluster %s has been deleted.", clusterName)
	if err := c.RemoveFinalizer(ctx, vc); err != nil {
		return fmt.Errorf("remove finalizer error: %v", err)
	}
	return nil
}

func (c *VirtualClusterJoinController) InitTargetKubeclient(kubeconfigStream []byte) (error, versioned.Interface, kubernetes.Interface, extensionsclient.Interface) {
	//targetKubeconfig := path.Join(DefaultKubeconfigPath, "kubeconfig")
	//config, err := utils.RestConfig(targetKubeconfig, "")
	config, err := utils.NewConfigFromBytes(kubeconfigStream)
	if err != nil {
		return fmt.Errorf("generate kubernetes config failed: %s", err), nil, nil, nil
	}

	kosmosClient, err := versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("generate Kosmos client failed: %v", err), nil, nil, nil
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("generate K8s basic client failed: %v", err), nil, nil, nil
	}

	k8sExtensionsClient, err := extensionsclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("generate K8s extensions client failed: %v", err), nil, nil, nil
	}

	return nil, kosmosClient, k8sClient, k8sExtensionsClient
}

func (c *VirtualClusterJoinController) DeployKosmos(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Start creating kosmos-clustertree secret %s/%s-clustertree-cluster-manager", request.Namespace, request.Name)
	clustertreeSecret, err := kosmosctl.GenerateSecret(treemanifest.ClusterTreeClusterManagerSecret, treemanifest.SecretReplace{
		Namespace:  request.Namespace,
		Cert:       cert.GetCrtEncode(),
		Key:        cert.GetKeyEncode(),
		Kubeconfig: vc.Spec.Kubeconfig,
		Name:       request.Name,
	})
	if err != nil {
		return err
	}
	err = c.Create(ctx, clustertreeSecret)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("install clustertree error, secret %s/%s-clustertree-cluster-manager created failed: %v",
				request.Namespace, request.Name, err)
		}
	}
	klog.Infof("Secret %s/%s-clustertree-cluster-manager has been created. ", request.Namespace, request.Name)

	klog.Infof("Start creating kosmos-clustertree deployment %s/%s-clustertree-cluster-manager...", request.Namespace, request.Name)
	imageRepository := os.Getenv(constants.DefauleImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}

	imageVersion := "v0.3.0" //os.Getenv(constants.DefauleImageVersionEnv)
	if len(imageVersion) == 0 {
		imageVersion = fmt.Sprintf("v%s", version.GetReleaseVersion().PatchRelease())
	}
	clustertreeDeploy, err := kosmosctl.GenerateDeployment(treemanifest.ClusterTreeClusterManagerDeployment, treemanifest.DeploymentReplace{
		Namespace:       request.Namespace,
		ImageRepository: imageRepository,
		Version:         imageVersion,
		FilePath:        treemanifest.DefaultKubeconfigPath,
		Name:            request.Name,
	})
	if err != nil {
		return err
	}
	err = c.Create(ctx, clustertreeDeploy)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("install clustertree error, deployment %s/%s-clustertree-cluster-manager created failed: %v",
				request.Namespace, request.Name, err)
		}
	}
	klog.Infof("Deployment %s/%s-clustertree-cluster-manager has been created. ", request.Namespace, request.Name)
	return nil
}

func (c *VirtualClusterJoinController) CreateCluster(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Attempting to create kosmos-clustertree CRDs for virtualcluster %s/%s...", request.Namespace, request.Name)
	clustertreeCluster, err := util.GenerateCustomResourceDefinition(manifest.Cluster, nil)
	if err != nil {
		return err
	}

	kubeconfigStream, err := base64.StdEncoding.DecodeString(vc.Spec.Kubeconfig)
	if err != nil {
		return fmt.Errorf("decode target kubernetes kubeconfig %s err: %v", vc.Spec.Kubeconfig, err)
	}
	err, kosmosClient, _, k8sExtensionsClient := c.InitTargetKubeclient(kubeconfigStream)
	if err != nil {
		return fmt.Errorf("crd kubernetes client failed: %v", err)
	}
	_, err = k8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), clustertreeCluster, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("CRD %s is existed, creation process will skip", clustertreeCluster.Name)
		} else {
			return fmt.Errorf("CRD create failed for virtualcluster %s/%s: %v", request.Namespace, request.Name, err)
		}
	}
	klog.Infof("Create CRD %s for virtualcluster %s/%s successful.", clustertreeCluster.Name, request.Namespace, request.Name)

	clusterName := fmt.Sprintf("virtualcluster-%s-%s", request.Namespace, request.Name)
	klog.Infof("Attempting to create cluster %s for %s/%s ...", clusterName, request.Namespace, request.Name)

	cluster := v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: v1alpha1.ClusterSpec{
			Kubeconfig: c.KubeconfigStream,
			Namespace:  request.Namespace,
			ClusterLinkOptions: &v1alpha1.ClusterLinkOptions{
				Enable:      false,
				NetworkType: v1alpha1.NetWorkTypeGateWay,
				IPFamily:    v1alpha1.IPFamilyTypeALL,
			},
			ClusterTreeOptions: &v1alpha1.ClusterTreeOptions{
				Enable: true,
			},
		},
	}

	hostK8sClient, err := utils.NewClientFromBytes(c.KubeconfigStream)
	if err != nil {
		return fmt.Errorf("crd kubernetes client failed: %v", err)
	}
	var leafModels []v1alpha1.LeafModel
	for _, nodeName := range vc.Spec.PromoteResources.Nodes {
		_, err := hostK8sClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("node %s doesn't exits: %v", nodeName, err)
			continue
		}
		leafModel := v1alpha1.LeafModel{
			LeafNodeName: nodeName,
			Taints: []corev1.Taint{
				{
					Effect: utils.KosmosNodeTaintEffect,
					Key:    utils.KosmosNodeTaintKey,
					Value:  utils.KosmosNodeValue,
				},
			},
			NodeSelector: v1alpha1.NodeSelector{
				NodeName: nodeName,
			},
		}
		leafModels = append(leafModels, leafModel)
	}
	cluster.Spec.ClusterTreeOptions.LeafModels = leafModels

	old, err := kosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = kosmosClient.KosmosV1alpha1().Clusters().Create(context.TODO(), &cluster, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create cluster %s failed: %v", clusterName, err)
			}
		} else {
			return fmt.Errorf("create cluster %s failed when get it first: %v", clusterName, err)
		}
	} else {
		cluster.ResourceVersion = old.GetResourceVersion()
		update, err := kosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), &cluster, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update cluster %s failed: %v", clusterName, err)
		} else {
			klog.Infof("Cluster %s hase been updated.", clusterName)
		}
		if !update.DeletionTimestamp.IsZero() {
			return fmt.Errorf("cluster %s is deleteting, need requeue", clusterName)
		}
	}
	klog.Infof("Cluster %s for %s/%s has been created.", clusterName, request.Namespace, request.Name)
	return nil
}

func (c *VirtualClusterJoinController) AddFinalizer(ctx context.Context, vc *v1alpha1.VirtualCluster) error {
	vcNew := vc.DeepCopy()
	if controllerutil.AddFinalizer(vcNew, constants.VirtualClusterFinalizerName) {
		err := c.Update(ctx, vcNew)
		if err != nil {
			return fmt.Errorf("add finalizer error for virtualcluster %s: %v", vc.Name, err)
		}
	}
	klog.Infof("add finalizer for virtualcluster %s", vc.Name)
	return nil
}

func (c *VirtualClusterJoinController) RemoveFinalizer(ctx context.Context, vc *v1alpha1.VirtualCluster) error {
	vcNew := vc.DeepCopy()
	if controllerutil.ContainsFinalizer(vcNew, constants.VirtualClusterFinalizerName) {
		controllerutil.RemoveFinalizer(vcNew, constants.VirtualClusterFinalizerName)
		err := c.Update(ctx, vcNew)
		if err != nil {
			return fmt.Errorf("remove finalizer error for virtualcluster %s: %v", vc.Name, err)
		}
	}
	klog.Infof("remove finalizer for virtualcluster %s", vc.Name)
	return nil
}

func (c *VirtualClusterJoinController) InstallClusterTree(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Start creating kosmos-clustertree in namespace %s", request.Namespace)
	defer klog.Infof("Finish creating kosmos-clustertree in namespace %s", request.Namespace)
	if err := c.DeployKosmos(ctx, request, vc); err != nil {
		return fmt.Errorf("deploy kosmos error: %v", err)
	}
	if err := c.CreateCluster(ctx, request, vc); err != nil {
		return fmt.Errorf("create cluster error: %v", err)
	}
	if err := c.AddFinalizer(ctx, vc); err != nil {
		return fmt.Errorf("add finalizer error: %v", err)
	}
	return nil
}

func (c *VirtualClusterJoinController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", constants.JoinControllerName, request.Name)
	defer klog.V(4).Infof("============ %s reconcile finish %s ============", constants.JoinControllerName, request.Name)
	var vc v1alpha1.VirtualCluster

	if err := c.Get(ctx, request.NamespacedName, &vc); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	if vc.DeletionTimestamp.IsZero() {
		if vc.Status.Phase != constants.VirtualClusterStatusCompleted {
			klog.Infof("cluster's status is %s, skip", vc.Status.Phase)
			return reconcile.Result{}, nil
		}
		err := c.InstallClusterTree(ctx, request, &vc)
		if err != nil {
			klog.Errorf("install %s error: %v", request.NamespacedName, err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
	} else {
		err := c.UninstallClusterTree(ctx, request, &vc)
		if err != nil {
			klog.Errorf("uninstall %s error: %v", request.NamespacedName, err)
			return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
		}
	}

	return reconcile.Result{}, nil
}

func (c *VirtualClusterJoinController) SetupWithManager(mgr manager.Manager) error {
	if c.Client == nil {
		c.Client = mgr.GetClient()
	}

	skipFunc := func(obj client.Object) bool {
		// skip reservedNS
		return obj.GetNamespace() != utils.ReservedNS
	}
	kubeconfigStream, err := os.ReadFile(c.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("read kubeconfig file failed: %v", err)
	}
	c.KubeconfigStream = kubeconfigStream

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.JoinControllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.VirtualCluster{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return skipFunc(createEvent.Object)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				if !skipFunc(updateEvent.ObjectNew) {
					return true
				}
				newObj := updateEvent.ObjectNew.(*v1alpha1.VirtualCluster)
				oldObj := updateEvent.ObjectOld.(*v1alpha1.VirtualCluster)

				if !newObj.DeletionTimestamp.IsZero() {
					return true
				}

				return !reflect.DeepEqual(newObj.Spec, oldObj.Spec) ||
					newObj.Status.Phase != oldObj.Status.Phase
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return skipFunc(deleteEvent.Object)
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				// TODO
				return false
			},
		})).
		Complete(c)
}
