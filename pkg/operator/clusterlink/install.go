package clusterlink

import (
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/agent"
	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/elector"
	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/global"
	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/manager"
	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/option"
	"github.com/kosmos.io/kosmos/pkg/operator/clusterlink/proxy"
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
