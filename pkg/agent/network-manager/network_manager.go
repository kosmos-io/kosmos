package networkmanager

import (
	"fmt"
	"sync"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

var lock = &sync.RWMutex{}

type NodeConfigSyncStatus string

const (
	NodeConfigSyncSuccess   NodeConfigSyncStatus = "success"
	NodeConfigSyncException NodeConfigSyncStatus = "exception"
)

type NetworkManager struct {
	NetworkInterface network.NetWork
	ToConfig         *clusterlinkv1alpha1.NodeConfigSpec
	FromConfig       *clusterlinkv1alpha1.NodeConfigSpec
	Status           NodeConfigSyncStatus
	Reason           string
}

type ConfigDiff struct {
	deleteConfig *clusterlinkv1alpha1.NodeConfigSpec
	// updateConfig *clusterlinkv1alpha1.NodeConfigSpec
	createConfig *clusterlinkv1alpha1.NodeConfigSpec
}

func NewNetworkManager(network network.NetWork) *NetworkManager {
	return &NetworkManager{
		NetworkInterface: network,
		ToConfig:         &clusterlinkv1alpha1.NodeConfigSpec{},
		FromConfig:       &clusterlinkv1alpha1.NodeConfigSpec{},
	}

}

/**
*  isSame, deleteconfig  createconfig
 */
func compareFunc[T any](old, new []T, f func(T, T) bool) (bool, []T, []T) {
	if old == nil && new == nil {
		return true, nil, nil
	}

	if old == nil {
		return false, []T{}, new
	}

	if new == nil {
		return false, old, []T{}
	}

	// old => new  delete
	deleteRecord := []T{}

	for _, o := range old {
		has := false
		for _, n := range new {
			if f(o, n) {
				has = true
				break
			}
		}
		if !has {
			deleteRecord = append(deleteRecord, o)
		}
	}

	// new => old  create
	createRecord := []T{}

	for _, o := range new {
		has := false
		for _, n := range old {
			if f(o, n) {
				has = true
				break
			}
		}
		if !has {
			createRecord = append(createRecord, o)
		}
	}
	return len(deleteRecord) == len(createRecord) && len(createRecord) == 0, deleteRecord, createRecord
}

func (e *NetworkManager) Diff(oldConfig, newConfig *clusterlinkv1alpha1.NodeConfigSpec) (bool, *clusterlinkv1alpha1.NodeConfigSpec, *clusterlinkv1alpha1.NodeConfigSpec) {
	deleteConfig := &clusterlinkv1alpha1.NodeConfigSpec{}
	createConfig := &clusterlinkv1alpha1.NodeConfigSpec{}
	isSame := true
	// devices:
	if flag, deleteRecord, createRecord := compareFunc(oldConfig.Devices, newConfig.Devices, func(a, b clusterlinkv1alpha1.Device) bool {
		return a.Compare(b)
	}); !flag {
		deleteConfig.Devices = deleteRecord
		createConfig.Devices = createRecord
		isSame = false
	}
	// routes:
	if flag, deleteRecord, createRecord := compareFunc(oldConfig.Routes, newConfig.Routes, func(a, b clusterlinkv1alpha1.Route) bool {
		return a.Compare(b)
	}); !flag {
		deleteConfig.Routes = deleteRecord
		createConfig.Routes = createRecord
		isSame = false
	}
	// iptables:
	if flag, deleteRecord, createRecord := compareFunc(oldConfig.Iptables, newConfig.Iptables, func(a, b clusterlinkv1alpha1.Iptables) bool {
		return a.Compare(b)
	}); !flag {
		deleteConfig.Iptables = deleteRecord
		createConfig.Iptables = createRecord
		isSame = false
	}
	// fdbs:
	if flag, deleteRecord, createRecord := compareFunc(oldConfig.Fdbs, newConfig.Fdbs, func(a, b clusterlinkv1alpha1.Fdb) bool {
		return a.Compare(b)
	}); !flag {
		// filter ff:ff:ff:ff:ff:ff
		for _, dr := range deleteRecord {
			if dr.Mac != "ff:ff:ff:ff:ff:ff" {
				deleteConfig.Fdbs = append(deleteConfig.Fdbs, dr)
			}
		}
		createConfig.Fdbs = createRecord
		isSame = false
	}
	// arps:
	if flag, deleteRecord, createRecord := compareFunc(oldConfig.Arps, newConfig.Arps, func(a, b clusterlinkv1alpha1.Arp) bool {
		return a.Compare(b)
	}); !flag {
		for _, dr := range deleteRecord {
			if dr.Mac != "ff:ff:ff:ff:ff:ff" {
				deleteConfig.Arps = append(deleteConfig.Arps, dr)
			}
		}
		createConfig.Arps = createRecord
		isSame = false
	}
	return isSame, deleteConfig, createConfig
}

func (e *NetworkManager) LoadSystemConfig() (*clusterlinkv1alpha1.NodeConfigSpec, error) {
	return e.NetworkInterface.LoadSysConfig()
}

func (e *NetworkManager) WriteSys(configDiff *ConfigDiff) error {
	var errs error

	if configDiff.deleteConfig != nil {
		config := configDiff.deleteConfig
		if config.Arps != nil {
			if err := e.NetworkInterface.DeleteArps(config.Arps); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Fdbs != nil {
			if err := e.NetworkInterface.DeleteFdbs(config.Fdbs); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Iptables != nil {
			if err := e.NetworkInterface.DeleteIptables(config.Iptables); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Routes != nil {
			if err := e.NetworkInterface.DeleteRoutes(config.Routes); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Devices != nil {
			if err := e.NetworkInterface.DeleteDevices(config.Devices); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
	}

	if configDiff.createConfig != nil {
		config := configDiff.createConfig
		// must create device first
		if config.Devices != nil {
			if err := e.NetworkInterface.AddDevices(config.Devices); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Arps != nil {
			if err := e.NetworkInterface.AddArps(config.Arps); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Fdbs != nil {
			if err := e.NetworkInterface.AddFdbs(config.Fdbs); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Iptables != nil {
			if err := e.NetworkInterface.AddIptables(config.Iptables); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}
		if config.Routes != nil {
			if err := e.NetworkInterface.AddRoutes(config.Routes); err != nil {
				klog.Warning(err)
				errs = errors.Wrap(err, fmt.Sprint(errs))
			}
		}

	}

	return errs
}

func (e *NetworkManager) UpdateFromCRD(nodeConfig *clusterlinkv1alpha1.NodeConfig) NodeConfigSyncStatus {
	lock.Lock()
	defer lock.Unlock()
	if flag, _, _ := e.Diff(e.ToConfig, &nodeConfig.Spec); flag {
		return e.Status
	}
	// new config
	e.ToConfig = &nodeConfig.Spec
	return e.UpdateSync()
}

func (e *NetworkManager) GetReason() string {
	return e.Reason
}

func (e *NetworkManager) UpdateFromChecker() NodeConfigSyncStatus {
	lock.Lock()
	defer lock.Unlock()

	if e.ToConfig != nil {
		return e.UpdateSync()
	}
	return e.Status
}

func printNodeConfig(data *clusterlinkv1alpha1.NodeConfigSpec) {
	klog.Infof("device: ", data.Devices)
	klog.Infof("Arps: ", data.Arps)
	klog.Infof("Fdbs: ", data.Fdbs)
	klog.Infof("Iptables: ", data.Iptables)
	klog.Infof("Routes: ", data.Routes)
}

func (e *NetworkManager) UpdateSync() NodeConfigSyncStatus {

	// load sysconfig
	fromConfig, err := e.LoadSystemConfig()
	if err != nil {
		e.Status = NodeConfigSyncException
		e.Reason = fmt.Sprintf("%s", err)
		return e.Status
	}

	flag, deleteConfig, createConfig := e.Diff(fromConfig, e.ToConfig)

	klog.Infoln("deleteConfig-------------------------------------------------")
	printNodeConfig(deleteConfig)
	klog.Infoln("createConfig-------------------------------------------------")
	printNodeConfig(createConfig)
	if flag {
		klog.Info("the config is same")
		return NodeConfigSyncSuccess
	}

	err = e.WriteSys(&ConfigDiff{
		deleteConfig: deleteConfig,
		createConfig: createConfig,
	})
	if err != nil {
		e.Status = NodeConfigSyncException
		e.Reason = fmt.Sprintf("%s", err)
		return e.Status
	}

	e.Status = NodeConfigSyncSuccess
	e.Reason = ""

	return NodeConfigSyncSuccess

}
