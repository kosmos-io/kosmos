package register

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type RootControllerOptions struct {
	Ctx                 context.Context
	Mgr                 manager.Manager
	RootKosmosClient    kosmosversioned.Interface
	RootDynamic         dynamic.Interface
	RootClient          kubernetes.Interface
	Options             *options.Options
	RootResourceManager *utils.ResourceManager
	GlobalLeafManager   leafUtils.LeafResourceManager
}

var RootControllers = map[string]func(*RootControllerOptions) error{}
var rootControllerMapLock sync.Mutex

type LeafControllerOptions struct {
	Mgr                 manager.Manager
	RootClient          client.Client
	RootClientSet       kubernetes.Interface
	LeafClientSet       kubernetes.Interface
	Node                *corev1.Node
	RootResourceManager *utils.ResourceManager
	RootKosmosClient    kosmosversioned.Interface
	Options             *options.Options
}

var LeafControllers = map[string]func(*LeafControllerOptions) error{}
var leafControllerMapLock sync.Mutex

func RegisterRootController(controllerName string, f func(*RootControllerOptions) error) {
	rootControllerMapLock.Lock()
	defer rootControllerMapLock.Unlock()

	if RootControllers[controllerName] != nil {
		klog.Fatalf("regist controller func error, the controller(%s) is registered repeatedly", controllerName)
		panic(fmt.Errorf("regist controller func error, the controller(%s) is registered repeatedly", controllerName))
	}

	RootControllers[controllerName] = f
}

func attachControllerToManager[T RootControllerOptions | LeafControllerOptions](mp map[string]func(*T) error, opt *T) error {
	for _, f := range mp {
		err := f(opt)
		if err != nil {
			return err
		}
	}
	return nil
}

func AttachRootControllerToManager(opt *RootControllerOptions) error {
	return attachControllerToManager(RootControllers, opt)
}

func RegisterLeafController(controllerName string, f func(*LeafControllerOptions) error) {
	leafControllerMapLock.Lock()
	defer leafControllerMapLock.Unlock()

	if LeafControllers[controllerName] != nil {
		klog.Fatalf("regist controller func error, the controller(%s) is registered repeatedly", controllerName)
		panic(fmt.Errorf("regist controller func error, the controller(%s) is registered repeatedly", controllerName))
	}

	LeafControllers[controllerName] = f
}

func AttachLeafControllerToManager(opt *LeafControllerOptions) error {
	return attachControllerToManager(LeafControllers, opt)
}
