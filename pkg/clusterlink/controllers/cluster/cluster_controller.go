package cluster

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	calicoclient "github.com/projectcalico/calico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/calico/libcalico-go/lib/options"
	cwatch "github.com/projectcalico/calico/libcalico-go/lib/watch"
	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

// KubeFlannelNetworkConfig
const (
	FlannelCNI             = "flannel"
	CiliumCNI              = "cilium"
	KubeFlannelConfigMap   = "kube-flannel-cfg"
	KubeCiliumConfigMap    = "cilium-config"
	KubeFlannelNetworkConf = "net-conf.json"
	KubeFlannelIPPool      = "Network"
)

type SetClusterPodCIDRFun func(cluster *clusterlinkv1alpha1.Cluster) error

type Controller struct {
	// RateLimiterOptions is the configuration for rate limiter which may significantly influence the performance of
	// the Controller.
	RateLimiterOptions   lifted.RateLimitOptions
	clusterName          string
	processor            lifted.AsyncWorker
	kubeClient           *kubernetes.Clientset
	dynamicClient        *dynamic.DynamicClient
	podLister            v1.PodLister
	clusterLinkClient    versioned.Interface
	setClusterPodCIDRFun SetClusterPodCIDRFun
	stopCh               <-chan struct{}
}

func NewController(clusterName string, kubeClient *kubernetes.Clientset, dynamicClient *dynamic.DynamicClient, clusterLinkClient versioned.Interface) *Controller {
	return &Controller{
		clusterName:       clusterName,
		kubeClient:        kubeClient,
		clusterLinkClient: clusterLinkClient,
		dynamicClient:     dynamicClient,
	}
}

func (c *Controller) onChange(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	c.processor.Enqueue(runtimeObj)
}

func (c *Controller) Start(ctx context.Context) error {
	klog.Infof("Starting cluster Controller.")
	c.stopCh = ctx.Done()

	opt := lifted.WorkerOptions{
		Name: "cluster Controller",
		KeyFunc: func(obj interface{}) (lifted.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      c.Reconcile,
		RateLimiterOptions: c.RateLimiterOptions,
	}
	c.processor = lifted.NewAsyncWorker(opt)

	factory := informers.NewSharedInformerFactory(c.kubeClient, 0)
	informer := factory.Core().V1().Pods().Informer()
	c.podLister = factory.Core().V1().Pods().Lister()
	podFilterFunc := func(pod *corev1.Pod) bool {
		return pod.Labels["component"] == "kube-apiserver"
	}

	cluster, err := c.clusterLinkClient.KosmosV1alpha1().Clusters().Get(ctx, c.clusterName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("can not find local cluster %s, err: %v", c.clusterName, err)
		return err
	}
	_, err = informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pod := obj.(*corev1.Pod)
			return podFilterFunc(pod)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.onChange(cluster)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.onChange(cluster)
			},
		},
	})
	if err != nil {
		klog.Errorf("can not add handler err: %v", err)
		return err
	}

	if cluster.Spec.ClusterLinkOptions.CNI == CiliumCNI {
		c.setClusterPodCIDRFun, err = c.initCiliumInformer(ctx, cluster, c.kubeClient)
		if err != nil {
			klog.Errorf("cluster %s initCiliumInformer err: %v", err)
			return err
		}
	} else if cluster.Spec.ClusterLinkOptions.CNI == FlannelCNI {
		c.setClusterPodCIDRFun, err = c.initFlannelInformer(ctx, cluster, c.kubeClient)
		if err != nil {
			klog.Errorf("cluster %s initCalicoInformer err: %v", err)
			return err
		}
	} else {
		isEtcd := CheckIsEtcd(cluster)
		if !isEtcd {
			c.setClusterPodCIDRFun, err = c.initCalicoInformer(ctx, cluster, c.dynamicClient)
			if err != nil {
				klog.Errorf("cluster %s initCalicoInformer err: %v", err)
				return err
			}
		} else {
			c.setClusterPodCIDRFun, err = c.initCalicoWatcherWithEtcdBackend(ctx, cluster)
			if err != nil {
				klog.Errorf("cluster %s initCalicoWatcherWithEtcdBackend err: %v", err)
				return err
			}
		}
	}
	factory.Start(c.stopCh)
	factory.WaitForCacheSync(c.stopCh)
	c.processor.Run(1, c.stopCh)
	<-ctx.Done()
	klog.Infof("Stop cluster controller as process done.")

	return nil
}

func (c *Controller) Reconcile(key lifted.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	namespacedName := types.NamespacedName{
		Name:      clusterWideKey.Name,
		Namespace: clusterWideKey.Namespace,
	}
	reconcileCluster, err := c.clusterLinkClient.KosmosV1alpha1().Clusters().Get(context.Background(), c.clusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("get %s error: %v", namespacedName, err)
		return err
	}
	// sync service cidr
	pods, err := c.podLister.Pods("kube-system").List(labels.Everything())
	if err != nil {
		klog.Errorf("list pod for cluster %s err: %v", reconcileCluster.Name, err)
		return err
	}
	var serviceCIDRS []string
	for i := range pods {
		pod := pods[i]
		if isAPIServer(pod) {
			serviceCIDRS, err = ResolveServiceCIDRs(pod)
			if err != nil {
				klog.Errorf("get %s service cidr error: %v", pod.Name, err)
				continue
			}
			break
		}
	}
	if len(serviceCIDRS) == 0 {
		klog.Errorf("resolve serviceCIDRS for cluster %s failure", c.clusterName)
		return err
	}
	// sync pod cidr
	err = c.setClusterPodCIDRFun(reconcileCluster)
	if err != nil {
		klog.Errorf("cluster %s sync pod cidr err: %v", reconcileCluster.GetName(), err)
		return err
	}

	reconcileCluster.Status.ClusterLinkStatus.ServiceCIDRs = serviceCIDRS
	//TODO use sub resource
	_, err = c.clusterLinkClient.KosmosV1alpha1().Clusters().Update(context.TODO(), reconcileCluster, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("could not update cluster %s, err: %v", reconcileCluster.GetName(), err)
		return err
	}
	return nil
}

func (c *Controller) initCalicoInformer(context context.Context, cluster *clusterlinkv1alpha1.Cluster, dynamicClient dynamic.Interface) (SetClusterPodCIDRFun, error) {
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	gvr := schema.GroupVersionResource{
		Group:    "crd.projectcalico.org",
		Version:  "v1",
		Resource: "ippools",
	}
	ippoolInformer := dynamicInformerFactory.ForResource(gvr)
	_, err := ippoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			runtimeObject, ok := obj.(runtime.Object)
			if !ok {
				return
			}
			metaInfo, err := meta.Accessor(runtimeObject)
			if err != nil {
				return
			}
			if !strings.HasPrefix(metaInfo.GetName(), utils.ExternalIPPoolNamePrefix) {
				c.onChange(cluster)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			runtimeObject, ok := newObj.(runtime.Object)
			if !ok {
				return
			}
			metaInfo, err := meta.Accessor(runtimeObject)
			if err != nil {
				return
			}
			if !strings.HasPrefix(metaInfo.GetName(), utils.ExternalIPPoolNamePrefix) {
				c.onChange(cluster)
			}
		},
		DeleteFunc: func(obj interface{}) {
			runtimeObject, ok := obj.(runtime.Object)
			if !ok {
				return
			}
			metaInfo, err := meta.Accessor(runtimeObject)
			if err != nil {
				return
			}
			if !strings.HasPrefix(metaInfo.GetName(), utils.ExternalIPPoolNamePrefix) {
				c.onChange(cluster)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	ippoolLister := ippoolInformer.Lister()
	dynamicInformerFactory.Start(context.Done())
	dynamicInformerFactory.WaitForCacheSync(context.Done())

	return func(cluster *clusterlinkv1alpha1.Cluster) error {
		ippools, err := ippoolLister.List(labels.Everything())
		if err != nil {
			return err
		}
		podCIDRS := make([]string, 0, 5)
		for i := range ippools {
			p := ippools[i]
			pool, ok := p.(*unstructured.Unstructured)
			if ok {
				name := pool.GetName()
				if !strings.HasPrefix(name, utils.ExternalIPPoolNamePrefix) {
					obj := pool.Object
					cidr := utils.GetCIDRs(obj)
					if len(cidr) > 0 {
						podCIDRS = append(podCIDRS, cidr)
					}
				}
			}
		}
		cluster.Status.ClusterLinkStatus.PodCIDRs = podCIDRS
		return nil
	}, nil
}

func (c *Controller) initCalicoWatcherWithEtcdBackend(ctx context.Context, cluster *clusterlinkv1alpha1.Cluster) (SetClusterPodCIDRFun, error) {
	calicoClient, err := GetCalicoClient(cluster)
	if err != nil {
		klog.Errorf("failed to get calico kubeClient %s err: %v", cluster.Name, err)
		return nil, err
	}

	watch, err := createIppoolWatcher(calicoClient, cluster)
	if err != nil {
		return nil, err
	}
	go func() {
	mainLoop:
		for {
			select {
			case <-ctx.Done():
				klog.Info("Context is done. Stop watching ippool in cluster %s", cluster.Name)
				watch.Stop()
				break mainLoop
			case event, ok := <-watch.ResultChan():
				if !ok {
					// If the channel is closed then resync/recreate the watch.
					klog.Info("Watch channel closed by remote - recreate watcher")
					watch.Stop()
					watch, err = createIppoolWatcher(calicoClient, cluster)
					if err != nil {
						klog.Errorf("create watcher for cluster %s err: %v", cluster.Name, err)
						klog.Info("sleep 10s retry")
						time.Sleep(10 * time.Second)
					}
				}
				switch event.Type {
				case cwatch.Error:
					klog.Errorf("receive err %v from cluster %s", err, cluster.Name)
					watch.Stop()
					watch, err = createIppoolWatcher(calicoClient, cluster)
					if err != nil {
						klog.Errorf("create watcher for cluster %s err: %v", cluster.Name, err)
						klog.Info("sleep 10s retry")
						time.Sleep(10 * time.Second)
					}
				case cwatch.Deleted:
					c.onChange(cluster)
				default:
					ippool, ok := event.Object.(*calicov3.IPPool)
					if ok {
						if validIPPool(ippool) {
							c.onChange(cluster)
						}
					} else {
						typeOfObject := reflect.TypeOf(event.Object)
						klog.Errorf("event object from cluster %s is not ippool type, get: %s", cluster.Name, typeOfObject.Name())
					}
				}
			}
		}
	}()
	return func(cluster *clusterlinkv1alpha1.Cluster) error {
		ippools, err := calicoClient.IPPools().List(context.Background(), options.ListOptions{})
		if err != nil {
			klog.Errorf("list ippools from cluster %s err: %v", cluster.Name, err)
			return err
		}
		podCIDRs := make([]string, 0, 5)
		for i := range ippools.Items {
			ippool := ippools.Items[i]
			if validIPPool(&ippool) {
				podCIDRs = append(podCIDRs, ippool.Spec.CIDR)
			}
		}
		cluster.Status.ClusterLinkStatus.PodCIDRs = podCIDRs
		return nil
	}, nil
}

// todo by wuyingjun-lucky
func (c *Controller) initFlannelInformer(context context.Context, cluster *clusterlinkv1alpha1.Cluster, kubeClient kubernetes.Interface) (SetClusterPodCIDRFun, error) {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	lister := informerFactory.Core().V1().ConfigMaps().Lister()
	_, err := informerFactory.Core().V1().ConfigMaps().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cm, ok := obj.(*corev1.ConfigMap)
				if !ok || cm.Name != KubeFlannelConfigMap {
					return
				}
				c.onChange(cluster)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				cm, ok := newObj.(*corev1.ConfigMap)
				if !ok || cm.Name != KubeFlannelConfigMap {
					return
				}
				c.onChange(cluster)
			},
			DeleteFunc: func(obj interface{}) {
				cm, ok := obj.(*corev1.ConfigMap)
				if !ok || cm.Name != KubeFlannelConfigMap {
					return
				}
				c.onChange(cluster)
			},
		})

	if err != nil {
		return nil, err
	}

	informerFactory.Start(context.Done())
	informerFactory.WaitForCacheSync(context.Done())

	return func(cluster *clusterlinkv1alpha1.Cluster) error {
		cms, err := lister.List(labels.Everything())
		if err != nil {
			return err
		}
		podCIDRS := make([]string, 0)
		for _, cm := range cms {
			if cm.Name == KubeFlannelConfigMap {
				v := cm.Data[KubeFlannelNetworkConf]
				if v == "" {
					continue
				}
				var data map[string]interface{}
				err = json.Unmarshal([]byte(v), &data)
				if err != nil {
					klog.Warningf("Convert kube flannel configmap network data err: %v", err)
					continue
				}

				PodIPPool, ok := data[KubeFlannelIPPool].(string)
				if !ok {
					klog.Warningf("Convert kube flannel configmap network data err to string")
					continue
				}

				podCIDRS = append(podCIDRS, PodIPPool)
				break
			}
		}
		cluster.Status.ClusterLinkStatus.PodCIDRs = podCIDRS
		return nil
	}, nil
}

func (c *Controller) initCiliumInformer(ctx context.Context, cluster *clusterlinkv1alpha1.Cluster, kubeClient *kubernetes.Clientset) (SetClusterPodCIDRFun, error) {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	lister := informerFactory.Core().V1().ConfigMaps().Lister()
	_, err := informerFactory.Core().V1().ConfigMaps().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				cm, ok := obj.(*corev1.ConfigMap)
				if !ok {
					return false
				}
				return cm.Name == KubeCiliumConfigMap && cm.Namespace == "kube-system"
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					c.onChange(cluster)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					c.onChange(cluster)
				},
				DeleteFunc: func(obj interface{}) {
					c.onChange(cluster)
				},
			},
		})

	if err != nil {
		return nil, err
	}
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
	return func(cluster *clusterlinkv1alpha1.Cluster) error {
		cm, err := lister.ConfigMaps("kube-system").Get(KubeCiliumConfigMap)
		if err != nil {
			return err
		}
		var podCIDRS []string
		ipv4CIDR, ok := cm.Data["cluster-pool-ipv4-cidr"]
		if !ok {
			klog.Warningf("cluster-pool-ipv4-cidr not found in cilium-config ConfigMap")
		}
		podCIDRS = append(podCIDRS, cast.ToStringSlice(ipv4CIDR)...)

		ipv6CIDR, ok := cm.Data["cluster-pool-ipv6-cidr"]
		if !ok {
			klog.Warningf("cluster-pool-ipv6-cidr not found in cilium-config ConfigMap")
		}
		podCIDRS = append(podCIDRS, cast.ToStringSlice(ipv6CIDR)...)

		cluster.Status.ClusterLinkStatus.PodCIDRs = podCIDRS
		return nil
	}, nil
}

func createIppoolWatcher(calicoClient calicoclient.Interface, cluster *clusterlinkv1alpha1.Cluster) (cwatch.Interface, error) {
	watch, err := calicoClient.IPPools().Watch(context.TODO(), options.ListOptions{})
	if err != nil {
		klog.Errorf("init calico ippool watch for cluster %s err: %v", cluster.Name, err)
		return nil, err
	}
	return watch, nil
}

func validIPPool(ippool *calicov3.IPPool) bool {
	return !ippool.Spec.Disabled && !strings.HasPrefix(utils.ExternalIPPoolNamePrefix, ippool.Name)
}

func isAPIServer(pod *corev1.Pod) bool {
	return pod.Namespace == "kube-system" && strings.HasPrefix(pod.Name, "kube-apiserver")
}
