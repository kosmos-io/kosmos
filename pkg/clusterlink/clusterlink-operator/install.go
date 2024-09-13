// nolint:revive
package clusterlink_operator

import (
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/agent"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/elector"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/global"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/manager"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/option"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/clusterlink-operator/proxy"
)

type AddonInstaller interface {
	Install(opt *option.AddonOption) error
	Uninstall(opt *option.AddonOption) error
}

var (
	installers = []AddonInstaller{global.New(), proxy.New(), agent.New(), elector.New(), manager.New()}
)

func Install(opt *option.AddonOption) error {
	klog.Infof("install clusterlink")
	for _, ins := range installers {
		if err := ins.Install(opt); err != nil {
			return err
		}
	}
	klog.Infof("install success")
	return nil
}

func Uninstall(opt *option.AddonOption) error {
	klog.Infof("uninstall clusterlink")
	i := len(installers)
	for i > 0 {
		i--
		if err := installers[i].Uninstall(opt); err != nil {
			return err
		}
	}
	klog.Infof("uninstall success")
	return nil
}
