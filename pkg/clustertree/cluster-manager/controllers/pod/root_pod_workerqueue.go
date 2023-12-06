package pod

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	rootpodsyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/root-pod"
	rootpodk8ssyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/root-pod/k8s"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/extensions/daemonset"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type RootPodWorkerQueueOption struct {
	Config            *rest.Config
	RootClient        kubernetes.Interface
	DynamicRootClient dynamic.Interface
	GlobalLeafManager leafUtils.LeafResourceManager
	Options           *options.Options
}

func NewRootPodWorkerQueue(opts *RootPodWorkerQueueOption) runtime.Controller {
	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	client, err := kubernetes.NewForConfig(opts.Config)
	if err != nil {
		klog.Fatal(err)
	}

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client,
		5*time.Second,
	)

	podInformer := podInformerFactory.Core().V1().Pods()

	eventFilter := func(obj interface{}) (bool, *corev1.Pod) {
		p, ok := obj.(*corev1.Pod)

		if !ok {
			klog.Fatal("convert pod error")
			return false, p
		}

		// skip reservedNS
		if p.GetNamespace() == utils.ReservedNS {
			return false, nil
		}
		// don't create pod if pod has label daemonset.kosmos.io/managed=""
		if _, ok := p.GetLabels()[daemonset.ManagedLabel]; ok {
			return false, nil
		}

		// p := obj.(*corev1.Pod)

		// skip daemonset
		if p.OwnerReferences != nil && len(p.OwnerReferences) > 0 {
			for _, or := range p.OwnerReferences {
				if or.Kind == "DaemonSet" {
					if p.Annotations != nil {
						if _, ok := p.Annotations[utils.KosmosDaemonsetAllowAnnotations]; ok {
							return true, p
						}
					}
					return false, nil
				}
			}
		}
		return true, p
	}

	_, err = podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if flag, pod := eventFilter(obj); flag {
				queue.Add(runtime.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				})
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			if flag, pod := eventFilter(old); flag {
				queue.Add(runtime.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				})
			}
		},
		DeleteFunc: func(obj interface{}) {
			if flag, pod := eventFilter(obj); flag {
				queue.Add(runtime.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				})
			}
		},
	})

	if err != nil {
		klog.Fatalf("add event handler error: %s", err)
		panic(err)
	}

	envResourceManager := rootpodk8ssyncers.NewEnvResourceManager(opts.DynamicRootClient)
	rootK8sK8sSyncer := &rootpodsyncers.RootPodSyncer{
		Client:             opts.RootClient,
		GlobalLeafManager:  opts.GlobalLeafManager,
		EnvResourceManager: envResourceManager,
		Options:            opts.Options,
		DynamicRootClient:  opts.DynamicRootClient,
	}

	return runtime.NewK8sWorkerQueue(queue, podInformer.Informer(), rootK8sK8sSyncer)
}
