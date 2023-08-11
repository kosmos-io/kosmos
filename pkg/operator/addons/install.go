package addons

import (
	"k8s.io/klog/v2"

	"cnp.io/clusterlink/pkg/operator/addons/agent"
	"cnp.io/clusterlink/pkg/operator/addons/elector"
	"cnp.io/clusterlink/pkg/operator/addons/global"
	"cnp.io/clusterlink/pkg/operator/addons/manager"
	"cnp.io/clusterlink/pkg/operator/addons/option"
)

type AddonInstaller interface {
	Install(opt *option.AddonOption) error
	Uninstall(opt *option.AddonOption) error
}

var (
	installers = []AddonInstaller{global.New(), agent.New(), elector.New(), manager.New()} // proxy.New()
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
