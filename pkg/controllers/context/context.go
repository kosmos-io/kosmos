package context

import (
	"context"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/clusterlink/pkg/utils/flags"
)

type Options struct {
	// Controllers contains all controller names.
	Controllers        []string
	ControlPanelConfig *rest.Config
	ClusterName        string
	RateLimiterOpts    flags.Options
}

// Context defines the context object for controller.
type Context struct {
	Mgr                         ctrl.Manager
	Opts                        Options
	Ctx                         context.Context
	ControlPlaneInformerManager externalversions.SharedInformerFactory
	ClusterLinkClient           versioned.Interface
}

// IsControllerEnabled check if a specified controller enabled or not.
func (c Context) IsControllerEnabled(name string, disabledByDefaultControllers sets.Set[string]) bool {
	hasStar := false
	for _, ctrl := range c.Opts.Controllers {
		if ctrl == name {
			return true
		}
		if ctrl == "-"+name {
			return false
		}
		if ctrl == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	if !hasStar {
		// nothing on by default
		return false
	}

	return !disabledByDefaultControllers.Has(name)
}

type CleanFunc func() error

// InitFunc is used to launch a particular controller.
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx Context) (enabled bool, cleanFunc CleanFunc, err error)

// Initializers is a public map of named controller groups
type Initializers map[string]InitFunc

// ControllerNames returns all known controller names
func (i Initializers) ControllerNames() []string {
	return sets.StringKeySet(i).List()
}

// StartControllers starts a set of controllers with a specified ControllerContext
func (i Initializers) StartControllers(ctx Context, controllersDisabledByDefault sets.Set[string]) ([]CleanFunc, error) {

	var cleanFuncs []CleanFunc
	for controllerName, initFn := range i {
		if !ctx.IsControllerEnabled(controllerName, controllersDisabledByDefault) {
			klog.Warningf("%q is disabled", controllerName)
			continue
		}
		klog.V(1).Infof("Starting %q", controllerName)
		started, cleanFun, err := initFn(ctx)
		if err != nil {
			klog.Errorf("Error starting %q", controllerName)
			return nil, err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
		if cleanFun != nil {
			cleanFuncs = append(cleanFuncs, cleanFun)
		}
		klog.Infof("Started %q", controllerName)
	}
	return cleanFuncs, nil
}
