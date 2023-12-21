package calicoippool

import (
	"context"
	"fmt"
	"strings"

	calicoapi "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/calico/libcalico-go/lib/clientv3"
	calicoerrors "github.com/projectcalico/calico/libcalico-go/lib/errors"
	"github.com/projectcalico/calico/libcalico-go/lib/options"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	helper "github.com/kosmos.io/kosmos/pkg/clusterlink/controllers/cluster"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
	"github.com/kosmos.io/kosmos/pkg/utils/net"
)

type ExternalIPPoolSet = map[ExternalClusterIPPool]struct{}

var calicoIPPoolGVR = schema.GroupVersionResource{
	Group:    "crd.projectcalico.org",
	Version:  "v1",
	Resource: "ippools",
}

type IPPoolClient interface {
	CreateOrUpdateCalicoIPPool(ipPools []*ExternalClusterIPPool) error
	DeleteIPPool(ipPools []*ExternalClusterIPPool) error
	ListIPPools() ([]*ExternalClusterIPPool, []IPPool, error)
}

type KubernetesBackend struct {
	dynamicClient dynamic.Interface
}

func (k *KubernetesBackend) ListIPPools() ([]*ExternalClusterIPPool, []IPPool, error) {
	list, err := k.dynamicClient.Resource(calicoIPPoolGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Infof("list ippool err: %v", err)
		return nil, nil, err
	}
	klog.V(4).Infof("cluster has %d ippool", len(list.Items))
	extIPPools := make([]*ExternalClusterIPPool, 0, 5)
	var ippools []IPPool
	for _, unPool := range list.Items {
		name := unPool.GetName()
		obj := unPool.Object
		ipType := getIPType(name)
		cidr := utils.GetCIDRs(obj)
		if strings.HasPrefix(name, utils.ExternalIPPoolNamePrefix) {
			clusterName := getClusterName(name, ipType)
			extIPPool := &ExternalClusterIPPool{cluster: clusterName, ipType: ipType, ipPool: cidr}
			extIPPools = append(extIPPools, extIPPool)
		} else {
			ippools = append(ippools, IPPool(cidr))
		}
	}
	return extIPPools, ippools, nil
}

func getClusterName(name string, ipType string) string {
	start := len(utils.ExternalIPPoolNamePrefix) + 1
	end := strings.LastIndex(name, ipType) - 1
	if end < 0 {
		klog.Errorf("GetClusterName(%s, %s) err", name, ipType)
		return ""
	}
	return name[start:end]
}

func getIPType(name string) string {
	var ipType string
	if strings.Contains(name, "service") {
		ipType = SERVICEIPType
	} else {
		ipType = PODIPType
	}
	return ipType
}

func (k *KubernetesBackend) CreateOrUpdateCalicoIPPool(ipPools []*ExternalClusterIPPool) error {
	var errs []error
	for _, ipPool := range ipPools {
		klog.Infof("calicoIPPool Controller will create or update ippool, %v", ipPool.String())
		ipPoolBytes, err := utils.ParseTemplate(CalicoIPPool, IPPoolReplace{
			Name:   genCalicoIPPoolName(ipPool.cluster, ipPool.ipType, ipPool.ipPool),
			IPPool: ipPool.ipPool,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("error when parsing ip pool template :%v", err))
		}

		ipPoolObj := &unstructured.Unstructured{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), ipPoolBytes, ipPoolObj); err != nil {
			errs = append(errs, fmt.Errorf("decode ippool error: %v", err))
		}

		_, err = k.dynamicClient.Resource(calicoIPPoolGVR).Create(context.TODO(), ipPoolObj, metav1.CreateOptions{})

		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				errs = append(errs, err)
				continue
			}
			continue
		}

		exist, err := k.dynamicClient.Resource(calicoIPPoolGVR).Get(context.TODO(), genCalicoIPPoolName(ipPool.cluster, ipPool.ipType, ipPool.ipPool), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, err)
			continue
		}

		ipPoolObj.SetResourceVersion(exist.GetResourceVersion())
		_, err = k.dynamicClient.Resource(calicoIPPoolGVR).Update(context.TODO(), ipPoolObj, metav1.UpdateOptions{})

		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

func (k *KubernetesBackend) DeleteIPPool(ipPools []*ExternalClusterIPPool) error {
	var errs []error
	for _, ipPool := range ipPools {
		klog.Infof("prepare delete ippool, %v", ipPool.String())
		err := k.dynamicClient.Resource(calicoIPPoolGVR).Delete(context.TODO(), genCalicoIPPoolName(ipPool.cluster, ipPool.ipType, ipPool.ipPool), metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

type EtcdBackend struct {
	calicoClient clientv3.Interface
}

func (e *EtcdBackend) CreateOrUpdateCalicoIPPool(ipPools []*ExternalClusterIPPool) error {
	var errs []error
	for _, ipPool := range ipPools {
		klog.Infof("prepare create or update ippool, %v", ipPool.String())
		pool := &calicoapi.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: genCalicoIPPoolName(ipPool.cluster, ipPool.ipType, ipPool.ipPool),
			},
			Spec: calicoapi.IPPoolSpec{
				CIDR:             ipPool.ipPool,
				NATOutgoing:      false,
				Disabled:         true,
				DisableBGPExport: true,
			},
		}
		p, err := e.calicoClient.IPPools().Get(context.TODO(), pool.Name, options.GetOptions{})
		if err != nil {
			klog.Errorf("get ippool %s err: %v", pool.Name, err)
		}
		if p != nil {
			_, err := e.calicoClient.IPPools().Update(context.TODO(), pool, options.SetOptions{})
			if err != nil {
				errs = append(errs, err)
				continue
			}
		} else {
			_, err := e.calicoClient.IPPools().Create(context.TODO(), pool, options.SetOptions{})
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

func (e *EtcdBackend) DeleteIPPool(ipPools []*ExternalClusterIPPool) error {
	var errs []error
	for _, ipPool := range ipPools {
		klog.Infof("prepare delete ippool, %v", ipPool.String())
		_, err := e.calicoClient.IPPools().Delete(context.TODO(), genCalicoIPPoolName(ipPool.cluster, ipPool.ipType, ipPool.ipPool), options.DeleteOptions{})
		if _, ok := err.(calicoerrors.ErrorResourceDoesNotExist); !ok {
			continue
		}
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

func (e *EtcdBackend) ListIPPools() ([]*ExternalClusterIPPool, []IPPool, error) {
	list, err := e.calicoClient.IPPools().List(context.TODO(), options.ListOptions{})
	if err != nil {
		klog.Errorf("list ippool err: %v", err)
		return nil, nil, err
	}
	extIPPools := make([]*ExternalClusterIPPool, 0, 5)
	var ippools []IPPool
	for _, ippool := range list.Items {
		if strings.HasPrefix(utils.ExternalIPPoolNamePrefix, ippool.Name) {
			ipType := getIPType(ippool.Name)
			extIPPools = append(extIPPools, &ExternalClusterIPPool{
				cluster: getClusterName(ippool.Name, ipType),
				ipType:  ipType,
				ipPool:  ippool.Spec.CIDR,
			})
		} else {
			ippools = append(ippools, IPPool(ippool.Spec.CIDR))
		}
	}
	return extIPPools, ippools, nil
}

type Controller struct {
	globalExtIPPoolSet ExternalIPPoolSet
	RateLimiterOptions flags.Options
	clusterName        string
	processor          utils.AsyncWorker
	clusterLinkClient  *versioned.Clientset
	clusterLister      v1alpha1.ClusterLister
	kubeClient         *kubernetes.Clientset
	dynamicClient      *dynamic.DynamicClient
	iPPoolClient       IPPoolClient
	stopCh             <-chan struct{}
}

func NewController(clusterName string, kubeClient *kubernetes.Clientset, dynamicClient *dynamic.DynamicClient, clusterLinkClient *versioned.Clientset) *Controller {
	return &Controller{
		globalExtIPPoolSet: map[ExternalClusterIPPool]struct{}{},
		clusterName:        clusterName,
		kubeClient:         kubeClient,
		dynamicClient:      dynamicClient,
		clusterLinkClient:  clusterLinkClient,
	}
}

// OnAdd handles object add event and push the object to queue.
func (c *Controller) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (c *Controller) OnUpdate(oldObj, newObj interface{}) {
	c.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (c *Controller) OnDelete(obj interface{}) {
	c.OnAdd(obj)
}

func (c *Controller) Start(ctx context.Context) error {
	klog.Infof("Starting CalicoIPPool Controller.")
	c.stopCh = ctx.Done()
	opt := utils.Options{
		Name: "cluster Controller",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      c.Reconcile,
		RateLimiterOptions: c.RateLimiterOptions,
	}
	c.processor = utils.NewAsyncWorker(opt)

	factory := externalversions.NewSharedInformerFactory(c.clusterLinkClient, 0)
	informer := factory.Kosmos().V1alpha1().Clusters().Informer()
	c.clusterLister = factory.Kosmos().V1alpha1().Clusters().Lister()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAdd,
		UpdateFunc: c.OnUpdate,
		DeleteFunc: c.OnDelete,
	})
	if err != nil {
		klog.Errorf("can not add handler err: %v", err)
		return err
	}
	factory.Start(c.stopCh)
	factory.WaitForCacheSync(c.stopCh)
	c.processor.Run(1, c.stopCh)
	<-ctx.Done()
	klog.Infof("Stop calicoippool controller as process done.")
	return nil
}

func (c *Controller) Reconcile(key utils.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}

	cluster, err := c.clusterLister.Get(clusterWideKey.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("get %s error: %v", clusterWideKey.Name, err)
		return err
	}

	klog.Infof("start reconcile cluster %s", cluster.Name)
	if cluster.Name == c.clusterName && cluster.Spec.ClusterLinkOptions.CNI != utils.CNITypeCalico {
		klog.Infof("cluster %s cni type is %s skip reconcile", cluster.Name, cluster.Spec.ClusterLinkOptions.CNI)
		return nil
	}
	for ipPool := range c.globalExtIPPoolSet {
		if ipPool.cluster == cluster.Name {
			delete(c.globalExtIPPoolSet, ipPool)
		}
	}
	getCIDR := func(cidr string, cidrMap map[string]string) string {
		if c, exist := cidrMap[cidr]; exist {
			return c
		} else {
			return cidr
		}
	}
	cidrMap := cluster.Spec.ClusterLinkOptions.GlobalCIDRsMap
	podCIDRS := cluster.Status.ClusterLinkStatus.PodCIDRs
	serviceCIDR := cluster.Status.ClusterLinkStatus.ServiceCIDRs
	for _, cidr := range podCIDRS {
		extIPPool := ExternalClusterIPPool{
			cluster: cluster.Name,
			ipType:  PODIPType,
			ipPool:  getCIDR(cidr, cidrMap),
		}
		c.globalExtIPPoolSet[extIPPool] = struct{}{}
	}
	for _, cidr := range serviceCIDR {
		extIPPool := ExternalClusterIPPool{
			cluster: cluster.Name,
			ipType:  SERVICEIPType,
			ipPool:  getCIDR(cidr, cidrMap),
		}
		c.globalExtIPPoolSet[extIPPool] = struct{}{}
	}
	klog.Infof("now has %d globalIPPools", len(c.globalExtIPPoolSet))
	if c.iPPoolClient == nil {
		if cluster.Name == c.clusterName {
			ipPoolClient, err := c.createIPPoolClient(cluster)
			if err != nil {
				klog.Errorf("create ipPoolClient err: %v", err)
				return err
			}
			c.iPPoolClient = ipPoolClient
		}
	}

	if c.iPPoolClient != nil {
		err = syncIPPool(c.clusterName, c.globalExtIPPoolSet, c.iPPoolClient)
		if err != nil {
			return err
		}
	}

	return nil
}

type IPPool string

type ExternalClusterIPPool struct {
	cluster string
	ipType  string
	ipPool  string
}

func (e *ExternalClusterIPPool) String() string {
	return fmt.Sprintf("cluster: %v, ipType: %v, ipPool %v", e.cluster, e.ipType, e.ipPool)
}

func syncIPPool(currentClusterName string, globalExtIPPoolSet ExternalIPPoolSet, client IPPoolClient) error {
	klog.Infof("cluster %s start sync ippool", currentClusterName)
	extIPPools, ippools, err := client.ListIPPools()
	if err != nil {
		return err
	}
	deleteIPPool := make([]*ExternalClusterIPPool, 0, 5)
	modifyIPPool := make([]*ExternalClusterIPPool, 0, 5)

	intersection := func(cidr1, cidr2 string) bool {
		var intersect bool
		if utils.IsIPv6(cidr1) && utils.IsIPv6(cidr2) {
			intersect = net.Intersect(cidr1, cidr2)
		} else {
			intersect = net.Intersect(cidr1, cidr2)
		}
		return intersect
	}

	klog.V(4).Infof("cluster %s ext ippool: %v", currentClusterName, extIPPools)
	clusterIPPoolSet := make(ExternalIPPoolSet)
	for _, pool := range extIPPools {
		clusterIPPoolSet[*pool] = struct{}{}
	}
	for g := range globalExtIPPoolSet {
		if g.cluster == currentClusterName {
			continue
		}
		for _, p := range ippools {
			intersect := intersection(g.ipPool, string(p))
			if intersect {
				klog.Warningf("%s has intersect with %s skip", g.ipPool, p)
				delete(globalExtIPPoolSet, g)
				continue
			}
		}
	}

	for pool := range globalExtIPPoolSet {
		poolCopy := &ExternalClusterIPPool{
			cluster: pool.cluster,
			ipType:  pool.ipType,
			ipPool:  pool.ipPool,
		}
		if pool.cluster == currentClusterName {
			klog.Infof("cluster %s skip %v", currentClusterName, poolCopy)
		} else {
			klog.Infof("cluster %s add pool %v to modifyIPPool", currentClusterName, poolCopy)
			modifyIPPool = append(modifyIPPool, poolCopy)
		}
	}

	for _, pool := range extIPPools {
		if _, ok := globalExtIPPoolSet[*pool]; !ok {
			deleteIPPool = append(deleteIPPool, pool)
		}
	}

	klog.V(4).Infof("cluster %s has %d ippools to delete, they are", currentClusterName, len(deleteIPPool), deleteIPPool)
	err = client.DeleteIPPool(deleteIPPool)
	if err != nil {
		klog.Errorf("cluster %s delete ippool err: %v", currentClusterName, err)
		return err
	}

	klog.V(4).Infof("cluster %s has %d ippools to modify, they are", currentClusterName, len(modifyIPPool), modifyIPPool)
	err = client.CreateOrUpdateCalicoIPPool(modifyIPPool)
	if err != nil {
		klog.Errorf("cluster %s modify ippool err: %v", currentClusterName, err)
		return err
	}

	return nil
}

func (c *Controller) createIPPoolClient(cluster *kosmosv1alpha1.Cluster) (IPPoolClient, error) {
	var ippoolClient IPPoolClient
	if helper.CheckIsEtcd(cluster) {
		client, err := helper.GetCalicoClient(cluster)
		if err != nil {
			return nil, err
		}
		ippoolClient = &EtcdBackend{
			calicoClient: client,
		}
	} else {
		ippoolClient = &KubernetesBackend{
			dynamicClient: c.dynamicClient,
		}
	}
	return ippoolClient, nil
}

func (c *Controller) CleanResource() error {
	if c.iPPoolClient == nil {
		cluster, err := c.clusterLinkClient.KosmosV1alpha1().Clusters().Get(context.TODO(), c.clusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		iPPoolClient, err := c.createIPPoolClient(cluster)
		if err != nil {
			return err
		}
		c.iPPoolClient = iPPoolClient
	}
	c.globalExtIPPoolSet = make(ExternalIPPoolSet)
	err := syncIPPool(c.clusterName, c.globalExtIPPoolSet, c.iPPoolClient)
	if err != nil {
		return fmt.Errorf("calicoippool_controller failed to clean resource: %v", err)
	}
	return nil
}
