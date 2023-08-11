package options

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kosmos.io/clusterlink/pkg/utils"
	"github.com/kosmos.io/clusterlink/pkg/utils/flags"
)

type ControllerManagerOptions struct {
	Controllers []string

	RateLimiterOpts flags.Options

	ControlPanelConfig string

	KubeConfig string

	ClusterName string
}

// NewControllerManagerOptions builds a default controller manager options.
func NewControllerManagerOptions() *ControllerManagerOptions {
	return &ControllerManagerOptions{}
}

func (o *ControllerManagerOptions) Validate() field.ErrorList {
	errs := field.ErrorList{}

	return errs
}

func (o *ControllerManagerOptions) AddFlags(fs *pflag.FlagSet, allControllers, disabledByDefaultControllers []string) {
	fs.StringSliceVar(&o.Controllers, "controllers", []string{"*"}, fmt.Sprintf(
		"A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. \nAll controllers: %s.\nDisabled-by-default controllers: %s",
		strings.Join(allControllers, ", "), strings.Join(disabledByDefaultControllers, ", "),
	))
	fs.StringVar(&o.ClusterName, "cluster", os.Getenv(utils.EnvClusterName), "current cluster name.")
	fs.StringVar(&o.ControlPanelConfig, "controlpanelconfig", "", "path to controlpanel kubeconfig file.")
}
