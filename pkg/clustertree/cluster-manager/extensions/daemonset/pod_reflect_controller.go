package daemonset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosinformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions/kosmos/v1alpha1"
	kosmoslister "github.com/kosmos.io/kosmos/pkg/generated/listers/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/flags"
	"github.com/kosmos.io/kosmos/pkg/utils/keys"
)

var KosmosDaemonSetKind = kosmosv1alpha1.SchemeGroupVersion.WithKind("DaemonSet")
var DaemonSetKind = appsv1.SchemeGroupVersion.WithKind("DaemonSet")

type PodReflectorController struct {
	// host cluster kube client
	kubeClient clientset.Interface

	dsLister appslisters.DaemonSetLister

	kdsLister kosmoslister.DaemonSetLister

	kNodeLister kosmoslister.KnodeLister

	podLister corelisters.PodLister

	// member cluster podManager map
	podManagerMap map[string]KNodePodManager

	daemonsetSynced cache.InformerSynced

	kdaemonsetSynced cache.InformerSynced

	kNodeSynced cache.InformerSynced

	podSynced cache.InformerSynced

	kNodeProcessor utils.AsyncWorker

	podProcessor utils.AsyncWorker

	rateLimiterOptions flags.Options

	lock sync.RWMutex
}

func NewPodReflectorController(kubeClient clientset.Interface,
	dsInformer appsv1informers.DaemonSetInformer,
	kdsInformer kosmosinformer.DaemonSetInformer,
	kNodeInformer kosmosinformer.KnodeInformer,
	podInformer corev1informers.PodInformer,
	rateLimiterOptions flags.Options,
) *PodReflectorController {
	pc := &PodReflectorController{
		kubeClient:         kubeClient,
		dsLister:           dsInformer.Lister(),
		kdsLister:          kdsInformer.Lister(),
		kNodeLister:        kNodeInformer.Lister(),
		podLister:          podInformer.Lister(),
		daemonsetSynced:    dsInformer.Informer().HasSynced,
		kdaemonsetSynced:   kdsInformer.Informer().HasSynced,
		kNodeSynced:        kNodeInformer.Informer().HasSynced,
		podSynced:          kNodeInformer.Informer().HasSynced,
		podManagerMap:      map[string]KNodePodManager{},
		rateLimiterOptions: rateLimiterOptions,
	}

	// nolint:errcheck
	kNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addKNode,
		DeleteFunc: pc.deleteKNode,
	})
	// nolint:errcheck
	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pod := obj.(*corev1.Pod)
			_, ok := pod.Annotations[ManagedLabel]
			return ok
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    pc.addPod,
			UpdateFunc: pc.updatePod,
			DeleteFunc: pc.deletePod,
		},
	})

	return pc
}

func (pc *PodReflectorController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.Infof("Starting pod reflector controller")
	defer klog.Infof("Shutting down pod reflector controller")

	knodeOpt := utils.Options{
		Name: "pod reflector controller: KNode",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			return keys.ClusterWideKeyFunc(obj)
		},
		ReconcileFunc:      pc.syncKNode,
		RateLimiterOptions: pc.rateLimiterOptions,
	}
	pc.kNodeProcessor = utils.NewAsyncWorker(knodeOpt)

	podOpt := utils.Options{
		Name: "pod reflector controller: pod",
		KeyFunc: func(obj interface{}) (utils.QueueKey, error) {
			pod := obj.(*corev1.Pod)
			cluster := getCluster(pod)
			if len(cluster) == 0 {
				return nil, fmt.Errorf("pod is not manage by kosmos daemon set")
			}
			return keys.FederatedKeyFunc(cluster, obj)
		},
		ReconcileFunc:      pc.syncPod,
		RateLimiterOptions: pc.rateLimiterOptions,
	}
	pc.podProcessor = utils.NewAsyncWorker(podOpt)
	if !cache.WaitForNamedCacheSync("pod_reflector_controller", ctx.Done(), pc.daemonsetSynced, pc.kdaemonsetSynced, pc.podSynced, pc.kNodeSynced) {
		klog.Errorf("Timed out waiting for caches to sync")
		return
	}
	pc.kNodeProcessor.Run(1, ctx.Done())
	pc.podProcessor.Run(1, ctx.Done())
}

func getCluster(pod *corev1.Pod) string {
	return pod.Annotations[ClusterAnnotationKey]
}

func (pc *PodReflectorController) syncKNode(key utils.QueueKey) error {
	pc.lock.Lock()
	defer pc.lock.Unlock()
	clusterWideKey, exist := key.(keys.ClusterWideKey)
	if !exist {
		klog.Errorf("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}
	name := clusterWideKey.Name
	knode, err := pc.kNodeLister.Get(name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("knode has been deleted %v", key)
			return nil
		}
		return err
	}
	manager, exist := pc.podManagerMap[knode.Name]
	if knode.DeletionTimestamp != nil {
		if exist {
			manager.Stop()
			delete(pc.podManagerMap, knode.Name)
		}
		return nil
	}

	if !exist {
		config, err := clientcmd.RESTConfigFromKubeConfig(knode.Spec.Kubeconfig)
		if err != nil {
			klog.Errorf("failed to create rest config for knode %s: %v", knode.Name, err)
			return err
		}
		kubeClient, err := clientset.NewForConfig(config)
		if err != nil {
			klog.Errorf("failed to create kube client for knode %s: %v", knode.Name, err)
		}
		kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
		podInformer := kubeFactory.Core().V1().Pods()
		// nolint:errcheck
		podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return false
				}
				_, ok = pod.Annotations[ManagedLabel]
				return ok
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    pc.addPod,
				UpdateFunc: pc.updatePod,
				DeleteFunc: pc.deletePod,
			},
		})
		manager = NewKNodePodManager(kubeClient, podInformer, kubeFactory)
		pc.podManagerMap[knode.Name] = manager
		manager.Start()
	}
	return nil
}

func (pc *PodReflectorController) syncPod(key utils.QueueKey) error {
	pc.lock.RLock()
	defer pc.lock.RUnlock()
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("invalid key type %T", key)
		return fmt.Errorf("invalid key")
	}
	knode := fedKey.Cluster
	name := fedKey.Name
	namespace := fedKey.Namespace
	manager, ok := pc.podManagerMap[knode]
	if !ok {
		msg := fmt.Sprintf("knode %s not found", knode)
		return errors.New(msg)
	}
	memberClusterPod, err := manager.GetPod(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// pod is not found in member cluster may be this pod has been deleted, try to delete from host cluster
			err := pc.kubeClient.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
			if !(err == nil && apierrors.IsNotFound(err)) {
				return err
			}
			return nil
		}
		klog.Errorf("failed to get pod %s/%s from member cluster %s: %v", namespace, name, knode, err)
		return err
	}
	if memberClusterPod.DeletionTimestamp != nil {
		err := pc.kubeClient.CoreV1().Pods(memberClusterPod.Namespace).Delete(context.Background(), memberClusterPod.Name, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			klog.Errorf("failed to delete pod %s/%s from member cluster %s: %v", namespace, name, knode, err)
			return err
		}
	}
	return pc.tryUpdateOrCreate(memberClusterPod)
}

func (pc *PodReflectorController) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	pc.podProcessor.Enqueue(pod)
}

func (pc *PodReflectorController) updatePod(old interface{}, new interface{}) {
	pod := new.(*corev1.Pod)
	pc.podProcessor.Enqueue(pod)
}

func (pc *PodReflectorController) deletePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	pc.podProcessor.Enqueue(pod)
}

func (pc *PodReflectorController) tryUpdateOrCreate(pod *corev1.Pod) error {
	clusterName := pod.Annotations[ClusterAnnotationKey]
	shadowPod, err := pc.podLister.Pods(pod.Namespace).Get(pod.Name)
	if err != nil {
		shadowPod, err = pc.kubeClient.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				newPod := pod.DeepCopy()
				err := pc.changeOwnerRef(newPod)
				if err != nil {
					klog.Errorf("failed to change owner ref for pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
					return err
				}
				newPod.ResourceVersion = ""
				newPod.Spec.NodeName = clusterName
				_, err = pc.kubeClient.CoreV1().Pods(newPod.Namespace).Create(context.Background(), newPod, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("failed to create pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
					return err
				}
				return nil
			}
			klog.Errorf("failed to get pod %s/%s: %v", pod.Namespace, pod.Name, err)
			return err
		}
	}
	copy := shadowPod.DeepCopy()
	copy.SetAnnotations(pod.Annotations)
	copy.SetLabels(pod.Labels)
	copy.Spec = pod.Spec
	copy.Spec.NodeName = clusterName
	copy.Status = pod.Status
	copy.UID = ""
	updated, err := pc.kubeClient.CoreV1().Pods(copy.Namespace).Update(context.Background(), copy, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return err
	}
	updated.Status = pod.Status
	_, err = pc.kubeClient.CoreV1().Pods(pod.Namespace).UpdateStatus(context.Background(), updated, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update pod %s/%s status: %v", pod.Namespace, pod.Name, err)
		return err
	}
	return nil
}

func (pc *PodReflectorController) addKNode(obj interface{}) {
	knode := obj.(*kosmosv1alpha1.Knode)
	pc.kNodeProcessor.Enqueue(knode)
}

func (pc *PodReflectorController) deleteKNode(obj interface{}) {
	knode := obj.(*kosmosv1alpha1.Knode)
	pc.kNodeProcessor.Enqueue(knode)
}

func (pc *PodReflectorController) changeOwnerRef(pod *corev1.Pod) error {
	var newOwnerReference []metav1.OwnerReference
	for i := range pod.OwnerReferences {
		ownRef := pod.OwnerReferences[i]
		if ownRef.Kind == "DaemonSet" {
			clusterName, ok := pod.Annotations[ClusterAnnotationKey]
			if !ok {
				continue
			}
			ownerName := strings.TrimRight(ownRef.Name, clusterName)
			daemonset, err := pc.dsLister.DaemonSets(pod.Namespace).Get(ownerName)
			if err != nil {
				return err
			}
			kdaemonset, err := pc.kdsLister.DaemonSets(pod.Namespace).Get(ownerName)
			if err != nil {
				return err
			}
			if kdaemonset != nil {
				newOwnerReference = append(newOwnerReference, metav1.OwnerReference{
					APIVersion: KosmosDaemonSetKind.Version,
					Kind:       KosmosDaemonSetKind.Kind,
					Name:       kdaemonset.Name,
					UID:        kdaemonset.UID,
				})
			}
			if daemonset != nil {
				_, isGlobalDs := daemonset.Annotations[MirrorAnnotation]
				if isGlobalDs {
					newOwnerReference = append(newOwnerReference, metav1.OwnerReference{
						APIVersion: DaemonSetKind.Version,
						Kind:       DaemonSetKind.Kind,
						Name:       daemonset.Name,
						UID:        daemonset.UID,
					})
				}
			}
			break
		}
	}
	pod.OwnerReferences = newOwnerReference
	return nil
}

type KNodePodManager struct {
	kubeClient clientset.Interface

	podLister corelisters.PodLister

	factory informers.SharedInformerFactory

	ctx context.Context

	cancelFun context.CancelFunc

	podSynced cache.InformerSynced
}

func (k *KNodePodManager) Start() {
	k.factory.Start(k.ctx.Done())
	if !cache.WaitForNamedCacheSync("pod reflect controller", k.ctx.Done(), k.podSynced) {
		klog.Errorf("failed to wait for pod caches to sync")
		return
	}
}

func (k *KNodePodManager) GetPod(namespace, name string) (*corev1.Pod, error) {
	pod, err := k.podLister.Pods(namespace).Get(name)
	if err != nil {
		pod, err = k.kubeClient.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return pod, nil
	}
	return pod.DeepCopy(), nil
}

func (k *KNodePodManager) Stop() {
	if k.cancelFun != nil {
		k.cancelFun()
	}
}

func (k KNodePodManager) GetPodLister() corelisters.PodLister {
	return k.podLister
}

func NewKNodePodManager(kubeClient clientset.Interface, podInformer corev1informers.PodInformer, factory informers.SharedInformerFactory) KNodePodManager {
	ctx, cancelFun := context.WithCancel(context.Background())
	return KNodePodManager{
		kubeClient: kubeClient,
		podLister:  podInformer.Lister(),
		factory:    factory,
		ctx:        ctx,
		cancelFun:  cancelFun,
		podSynced:  podInformer.Informer().HasSynced,
	}
}
