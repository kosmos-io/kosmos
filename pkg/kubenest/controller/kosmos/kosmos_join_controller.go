package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"sync"

	corev1 "k8s.io/api/core/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
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
	file "github.com/kosmos.io/kosmos/pkg/kosmosctl/manifest"
	kosmosctl "github.com/kosmos.io/kosmos/pkg/kosmosctl/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	manifest "github.com/kosmos.io/kosmos/pkg/kubenest/manifest/kosmos"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/version"
)

type KosmosJoinController struct {
	client.Client
	EventRecorder              record.EventRecorder
	KubeConfig                 *restclient.Config
	KubeconfigStream           []byte
	AllowNodeOwnbyMulticluster bool
}

var nodeOwnerMap = make(map[string]string)
var mu sync.Mutex
var once sync.Once

func (c *KosmosJoinController) RemoveClusterFinalizer(cluster *v1alpha1.Cluster, kosmosClient versioned.Interface) error {
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

func (c *KosmosJoinController) InitNodeOwnerMap() {
	vcList := &v1alpha1.VirtualClusterList{}
	err := c.List(context.Background(), vcList)
	if err != nil {
		klog.Errorf("list virtual cluster error: %v", err)
		return
	}
	for _, vc := range vcList.Items {
		if vc.Status.Phase == v1alpha1.Completed {
			kubeconfigStream, err := base64.StdEncoding.DecodeString(vc.Spec.Kubeconfig)
			if err != nil {
				klog.Errorf("virtualcluster %s decode target kubernetes kubeconfig %s err: %v", vc.Name, vc.Spec.Kubeconfig, err)
				continue
			}
			kosmosClient, _, k8sExtensionsClient, err := c.InitTargetKubeclient(kubeconfigStream)
			if err != nil {
				klog.Errorf("virtualcluster %s crd kubernetes client failed: %v", vc.Name, err)
				continue
			}
			_, err = k8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "clusters.kosmos.io", metav1.GetOptions{})
			if err != nil {
				if !apierrors.IsNotFound(err) {
					klog.Errorf("virtualcluster %s get crd clusters.kosmos.io err: %v", vc.Name, err)
				}
				klog.Infof("virtualcluster %s crd clusters.kosmos.io doesn't exist", vc.Name)
				continue
			}
			clusters, err := kosmosClient.KosmosV1alpha1().Clusters().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				if !apierrors.IsNotFound(err) {
					klog.Infof("virtualcluster %s get clusters err: %v", vc.Name, err)
				}
				klog.Infof("virtualcluster %s cluster doesn't exist", vc.Name)
				continue
			}
			mu.Lock()
			for _, cluster := range clusters.Items {
				for _, node := range cluster.Spec.ClusterTreeOptions.LeafModels {
					if vcName, ok := nodeOwnerMap[node.LeafNodeName]; ok && len(vcName) > 0 {
						klog.Warningf("node %s also belong to cluster %s", node.LeafNodeName, vcName)
					}
					nodeOwnerMap[node.LeafNodeName] = vc.Name
				}
			}
			mu.Unlock()
		}
		klog.Infof("check virtualcluster %s, nodeOwnerMap is %v", vc.Name, nodeOwnerMap)
	}
	klog.Infof("Init nodeOwnerMap is %v", nodeOwnerMap)
}

func (c *KosmosJoinController) UninstallClusterTree(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Start deleting kosmos-clustertree deployment %s/%s-clustertree-cluster-manager...", request.Namespace, request.Name)
	clustertreeDeploy, err := kosmosctl.GenerateDeployment(manifest.ClusterTreeClusterManagerDeployment, manifest.DeploymentReplace{
		Namespace:       request.Namespace,
		ImageRepository: "null",
		Version:         "null",
		Name:            request.Name,
		FilePath:        constants.DefaultKubeconfigPath,
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
	clustertreeSecret, err := kosmosctl.GenerateSecret(manifest.ClusterTreeClusterManagerSecret, manifest.SecretReplace{
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
	kosmosClient, _, _, err := c.InitTargetKubeclient(kubeconfigStream)
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
		if !c.AllowNodeOwnbyMulticluster {
			mu.Lock()
			for _, nodeName := range old.Spec.ClusterTreeOptions.LeafModels {
				nodeOwnerMap[nodeName.LeafNodeName] = ""
			}
			mu.Unlock()
		}
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

func (c *KosmosJoinController) InitTargetKubeclient(kubeconfigStream []byte) (versioned.Interface, kubernetes.Interface, extensionsclient.Interface, error) {
	//targetKubeconfig := path.Join(DefaultKubeconfigPath, "kubeconfig")
	//config, err := utils.RestConfig(targetKubeconfig, "")
	config, err := utils.NewConfigFromBytes(kubeconfigStream)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate kubernetes config failed: %s", err)
	}

	kosmosClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate Kosmos client failed: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate K8s basic client failed: %v", err)
	}

	k8sExtensionsClient, err := extensionsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate K8s extensions client failed: %v", err)
	}

	return kosmosClient, k8sClient, k8sExtensionsClient, nil
}

func (c *KosmosJoinController) DeployKosmos(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	klog.Infof("Start creating kosmos-clustertree secret %s/%s-clustertree-cluster-manager", request.Namespace, request.Name)
	clustertreeSecret, err := kosmosctl.GenerateSecret(manifest.ClusterTreeClusterManagerSecret, manifest.SecretReplace{
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
	imageRepository := os.Getenv(constants.DefaultImageRepositoryEnv)
	if len(imageRepository) == 0 {
		imageRepository = utils.DefaultImageRepository
	}

	//TODO: hard coded,modify in future
	imageVersion := "v0.3.0" //os.Getenv(constants.DefauleImageVersionEnv)
	if len(imageVersion) == 0 {
		imageVersion = fmt.Sprintf("v%s", version.GetReleaseVersion().PatchRelease())
	}
	clustertreeDeploy, err := kosmosctl.GenerateDeployment(manifest.ClusterTreeClusterManagerDeployment, manifest.DeploymentReplace{
		Namespace:       request.Namespace,
		ImageRepository: imageRepository,
		Version:         imageVersion,
		FilePath:        constants.DefaultKubeconfigPath,
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

func (c *KosmosJoinController) ClearSomeNodeOwner(nodeNames *[]string) {
	if !c.AllowNodeOwnbyMulticluster {
		mu.Lock()
		for _, nodeName := range *nodeNames {
			nodeOwnerMap[nodeName] = ""
		}
		mu.Unlock()
	}
}

func (c *KosmosJoinController) CreateClusterObject(ctx context.Context, request reconcile.Request,
	vc *v1alpha1.VirtualCluster, hostK8sClient kubernetes.Interface, cluster *v1alpha1.Cluster) (*[]string, *map[string]struct{}, error) {
	var leafModels []v1alpha1.LeafModel
	// recored new nodes' name, if error happen before create or update, need clear newNodeNames
	newNodeNames := []string{}
	// record all nodes' name in a map, when update cr, may need to delete some old node
	// compare all nodes in cluster cr to all node exits in virtual cluster,we can find which ndoe should be deleted
	allNodeNamesMap := map[string]struct{}{}

	for _, nodeInfo := range vc.Spec.PromoteResources.NodeInfos {
		_, err := hostK8sClient.CoreV1().Nodes().Get(context.Background(), nodeInfo.NodeName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Warningf("node %s doesn't exits: %v", nodeInfo.NodeName, err)
				continue
			}
			c.ClearSomeNodeOwner(&newNodeNames)
			klog.Errorf("get node %s error: %v", nodeInfo.NodeName, err)
			return nil, nil, err
		}
		if !c.AllowNodeOwnbyMulticluster {
			mu.Lock()
			if len(nodeOwnerMap) > 0 {
				if nodeOwner, existed := nodeOwnerMap[nodeInfo.NodeName]; existed && len(nodeOwner) > 0 {
					if nodeOwner != cluster.Name {
						continue
					}
				} else {
					newNodeNames = append(newNodeNames, nodeInfo.NodeName)
				}
			} else {
				newNodeNames = append(newNodeNames, nodeInfo.NodeName)
			}
			allNodeNamesMap[nodeInfo.NodeName] = struct{}{}
			nodeOwnerMap[nodeInfo.NodeName] = cluster.Name
			mu.Unlock()
		}
		leafModel := v1alpha1.LeafModel{
			LeafNodeName: nodeInfo.NodeName,
			Taints: []corev1.Taint{
				{
					Effect: utils.KosmosNodeTaintEffect,
					Key:    utils.KosmosNodeTaintKey,
					Value:  utils.KosmosNodeValue,
				},
			},
			NodeSelector: v1alpha1.NodeSelector{
				NodeName: nodeInfo.NodeName,
			},
		}
		leafModels = append(leafModels, leafModel)
	}
	klog.V(7).Infof("all new node in cluster %s: %v", cluster.Name, newNodeNames)
	klog.V(7).Infof("all node in cluster %s: %v", cluster.Name, allNodeNamesMap)
	cluster.Spec.ClusterTreeOptions.LeafModels = leafModels

	return &newNodeNames, &allNodeNamesMap, nil
}

func (c *KosmosJoinController) CreateOrUpdateCluster(ctx context.Context, request reconcile.Request,
	kosmosClient versioned.Interface, k8sClient kubernetes.Interface, newNodeNames *[]string,
	allNodeNamesMap *map[string]struct{}, cluster *v1alpha1.Cluster) error {
	old, err := kosmosClient.KosmosV1alpha1().Clusters().Get(context.TODO(), cluster.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = kosmosClient.KosmosV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
			if err != nil {
				c.ClearSomeNodeOwner(newNodeNames)
				return fmt.Errorf("create cluster %s failed: %v", cluster.Name, err)
			}
		} else {
			c.ClearSomeNodeOwner(newNodeNames)
			return fmt.Errorf("create cluster %s failed when get it first: %v", cluster.Name, err)
		}
		klog.Infof("Cluster %s for %s/%s has been created.", cluster.Name, request.Namespace, request.Name)
	} else {
		cluster.ResourceVersion = old.GetResourceVersion()
		_, err = kosmosClient.KosmosV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
		if err != nil {
			c.ClearSomeNodeOwner(newNodeNames)
			return fmt.Errorf("update cluster %s failed: %v", cluster.Name, err)
		}

		k8sNodesList, err := k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("list %s's k8s nodes error: %v", cluster.Name, err)
		}
		// clear node, delete some node not in new VirtualCluster.spec.PromoteResources.Nodes
		for _, node := range k8sNodesList.Items {
			if _, ok := (*allNodeNamesMap)[node.Name]; !ok {
				// if existed node not in map, it should be deleted
				err := k8sClient.CoreV1().Nodes().Delete(context.TODO(), node.Name, metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("delete %s's k8s nodes error: %v", cluster.Name, err)
				}
				// clear ndoe's owner
				mu.Lock()
				nodeOwnerMap[node.Name] = ""
				mu.Unlock()
			}
		}
		klog.Infof("Cluster %s for %s/%s has been updated.", cluster.Name, request.Namespace, request.Name)
	}
	return nil
}

func (c *KosmosJoinController) CreateCluster(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
	kubeconfigStream, err := base64.StdEncoding.DecodeString(vc.Spec.Kubeconfig)
	if err != nil {
		return fmt.Errorf("decode target kubernetes kubeconfig %s err: %v", vc.Spec.Kubeconfig, err)
	}
	kosmosClient, k8sClient, k8sExtensionsClient, err := c.InitTargetKubeclient(kubeconfigStream)
	if err != nil {
		return fmt.Errorf("crd kubernetes client failed: %v", err)
	}

	// create crd cluster.kosmos.io
	klog.Infof("Attempting to create kosmos-clustertree CRDs for virtualcluster %s/%s...", request.Namespace, request.Name)
	for _, crdToCreate := range []string{file.ServiceImport, file.Cluster,
		file.ServiceExport, file.ClusterPodConvert, file.PodConvert} {
		crdObject, err := kosmosctl.GenerateCustomResourceDefinition(crdToCreate, nil)
		if err != nil {
			return err
		}
		_, err = k8sExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), crdObject, metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("create CRD %s for virtualcluster %s/%s failed: %v",
					crdObject.Name, request.Namespace, request.Name, err)
			}
			klog.Warningf("CRD %s is existed, creation process will skip", crdObject.Name)
		} else {
			klog.Infof("Create CRD %s for virtualcluster %s/%s successful.", crdObject.Name, request.Namespace, request.Name)
		}
	}

	// construct cluster.kosmos.io cr
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

	newNodeNames, allNodeNamesMap, nil := c.CreateClusterObject(ctx, request, vc, hostK8sClient, &cluster)
	if err != nil {
		return err
	}

	// use client-go to create or update cluster.kosmos.io cr
	err = c.CreateOrUpdateCluster(ctx, request, kosmosClient, k8sClient, newNodeNames, allNodeNamesMap, &cluster)
	if err != nil {
		return err
	}

	return nil
}

func (c *KosmosJoinController) AddFinalizer(ctx context.Context, vc *v1alpha1.VirtualCluster) error {
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

func (c *KosmosJoinController) RemoveFinalizer(ctx context.Context, vc *v1alpha1.VirtualCluster) error {
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

func (c *KosmosJoinController) InstallClusterTree(ctx context.Context, request reconcile.Request, vc *v1alpha1.VirtualCluster) error {
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

func (c *KosmosJoinController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ %s starts to reconcile %s ============", constants.KosmosJoinControllerName, request.Name)
	defer klog.V(4).Infof("============ %s reconcile finish %s ============", constants.KosmosJoinControllerName, request.Name)
	if !c.AllowNodeOwnbyMulticluster {
		once.Do(c.InitNodeOwnerMap)
	}
	var vc v1alpha1.VirtualCluster

	if err := c.Get(ctx, request.NamespacedName, &vc); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Errorf("get %s error: %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: utils.DefaultRequeueTime}, nil
	}
	if vc.DeletionTimestamp.IsZero() {
		if vc.Status.Phase != v1alpha1.Completed {
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

func (c *KosmosJoinController) SetupWithManager(mgr manager.Manager) error {
	if c.Client == nil {
		c.Client = mgr.GetClient()
	}

	skipFunc := func(obj client.Object) bool {
		// skip reservedNS
		return obj.GetNamespace() != utils.ReservedNS
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.KosmosJoinControllerName).
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
