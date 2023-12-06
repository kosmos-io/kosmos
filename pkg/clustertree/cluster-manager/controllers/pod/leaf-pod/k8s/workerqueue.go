package k8s

import (
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	leafpodsyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

func NewLeafPodK8wWorkerQueue(opts *leafpodsyncers.LeafPodWorkerQueueOption) runtime.Controller {
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

		if len(p.Spec.NodeName) == 0 {
			return false, p
		}

		if p.GetNamespace() == utils.ReservedNS {
			return false, p
		}

		return podutils.IsKosmosPod(p), p
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
				if !cmp.Equal(old.(*corev1.Pod).Status, new.(*corev1.Pod).Status) {
					queue.Add(runtime.NamespacedName{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					})
				}
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

	leafClient, err := kubernetes.NewForConfig(opts.Config)
	if err != nil {
		klog.Fatalf("could not build clientset for cluster %s", err)
		panic(err)
	}

	leafK8sSyncer := &leafPodK8sSyncer{
		LeafClient: leafClient,
		RootClient: opts.RootClient,
	}

	return runtime.NewK8sWorkerQueue(queue, podInformer.Informer(), leafK8sSyncer)
}
