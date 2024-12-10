package options

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/lifted"
)

type ControllerManagerOptions struct {
	Controllers []string

	RateLimiterOpts lifted.RateLimitOptions

	ControlPanelConfig string

	ClusterName string

	utils.KubernetesOptions
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
	fs.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", utils.DefaultKubeQPS, "QPS to use while talking with kube-apiserver.")
	fs.IntVar(&o.KubernetesOptions.Burst, "kube-burst", utils.DefaultKubeBurst, "Burst to use while talking with kube-apiserver.")
	fs.StringSliceVar(&o.Controllers, "controllers", []string{"*"}, fmt.Sprintf(
		"A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. \nAll controllers: %s.\nDisabled-by-default controllers: %s",
		strings.Join(allControllers, ", "), strings.Join(disabledByDefaultControllers, ", "),
	))
	fs.StringVar(&o.ClusterName, "cluster", os.Getenv(utils.EnvClusterName), "current cluster name.")
	fs.StringVar(&o.ControlPanelConfig, "controlpanelconfig", "", "path to controlpanel kubeconfig file.")
}
