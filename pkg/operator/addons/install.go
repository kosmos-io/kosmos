package addons

import (
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/global"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/proxy"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/operator/addons/agent"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/elector"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/manager"
	"github.com/kosmos.io/clusterlink/pkg/operator/addons/option"
)

type AddonInstaller interface {
	Install(opt *option.AddonOption) error
	Uninstall(opt *option.AddonOption) error
}

var (
	installers = []AddonInstaller{global.New(), proxy.New(), agent.New(), elector.New(), manager.New()}
)

func Install(opt *option.AddonOption) error {
	klog.Infof("install addons")
	for _, ins := range installers {
		if err := ins.Install(opt); err != nil {
			return err
		}
	}
	klog.Infof("install success")
	return nil
}

func Uninstall(opt *option.AddonOption) error {
	klog.Infof("uninstall addons")
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
