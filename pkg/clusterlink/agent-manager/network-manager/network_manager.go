package networkmanager

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
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
	DeleteConfig *clusterlinkv1alpha1.NodeConfigSpec
	// updateConfig *clusterlinkv1alpha1.NodeConfigSpec
	CreateConfig *clusterlinkv1alpha1.NodeConfigSpec
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

	typeOfOldConfig := reflect.TypeOf(*oldConfig)
	valueOfDeleteConfig := reflect.ValueOf(deleteConfig).Elem()

	for i := 0; i < typeOfOldConfig.NumField(); i++ {
		fieldName := typeOfOldConfig.Field(i).Name
		fieldType := typeOfOldConfig.Field(i).Type
		valueByName := valueOfDeleteConfig.FieldByName(fieldName)

		var flag bool
		switch fieldName {
		case clusterlinkv1alpha1.DeviceName:
			flag, deleteConfig.Devices, createConfig.Devices =
				compareFunc(oldConfig.Devices, newConfig.Devices, func(a, b clusterlinkv1alpha1.Device) bool {
					return a.Compare(b)
				})
		case clusterlinkv1alpha1.RouteName:
			flag, deleteConfig.Routes, createConfig.Routes =
				compareFunc(oldConfig.Routes, newConfig.Routes, func(a, b clusterlinkv1alpha1.Route) bool {
					return a.Compare(b)
				})
		case clusterlinkv1alpha1.IptablesName:
			flag, deleteConfig.Iptables, createConfig.Iptables =
				compareFunc(oldConfig.Iptables, newConfig.Iptables, func(a, b clusterlinkv1alpha1.Iptables) bool {
					return a.Compare(b)
				})
		case clusterlinkv1alpha1.FdbName:
			flag, deleteConfig.Fdbs, createConfig.Fdbs =
				compareFunc(oldConfig.Fdbs, newConfig.Fdbs, func(a, b clusterlinkv1alpha1.Fdb) bool {
					return a.Compare(b)
				})
		case clusterlinkv1alpha1.ArpName:
			flag, deleteConfig.Arps, createConfig.Arps =
				compareFunc(oldConfig.Arps, newConfig.Arps, func(a, b clusterlinkv1alpha1.Arp) bool {
					return a.Compare(b)
				})
		}
		if !flag {
			isSame = flag
			if fieldName == clusterlinkv1alpha1.ArpName || fieldName == clusterlinkv1alpha1.FdbName {
				len := valueByName.Len()
				deleteSlice := reflect.MakeSlice(fieldType, len, len)
				for j := 0; j < len; j++ {
					fieldByName := valueByName.Index(j).FieldByName("Mac")
					if fieldByName.IsValid() {
						if fieldByName.Interface().(string) == "ff:ff:ff:ff:ff:ff" {
							continue
						}
					}
					deleteSlice.Index(j).Set(valueByName.Index(j))
				}
				valueByName.Set(deleteSlice)
			}
		}
	}
	return isSame, deleteConfig, createConfig
}

func (e *NetworkManager) LoadSystemConfig() (*clusterlinkv1alpha1.NodeConfigSpec, error) {
	return e.NetworkInterface.LoadSysConfig()
}

func (e *NetworkManager) delete(value reflect.Value, typeName string) error {
	var err error
	switch typeName {
	case clusterlinkv1alpha1.DeviceName:
		err = e.NetworkInterface.DeleteDevices(value.Interface().([]clusterlinkv1alpha1.Device))
	case clusterlinkv1alpha1.RouteName:
		err = e.NetworkInterface.DeleteRoutes(value.Interface().([]clusterlinkv1alpha1.Route))
	case clusterlinkv1alpha1.IptablesName:
		err = e.NetworkInterface.DeleteIptables(value.Interface().([]clusterlinkv1alpha1.Iptables))
	case clusterlinkv1alpha1.FdbName:
		err = e.NetworkInterface.DeleteFdbs(value.Interface().([]clusterlinkv1alpha1.Fdb))
	case clusterlinkv1alpha1.ArpName:
		err = e.NetworkInterface.DeleteArps(value.Interface().([]clusterlinkv1alpha1.Arp))
	default:
		err = fmt.Errorf("not found this value, name: %s", typeName)
	}
	return err
}

func (e *NetworkManager) add(value reflect.Value, typeName string) error {
	var err error
	switch typeName {
	case clusterlinkv1alpha1.DeviceName:
		err = e.NetworkInterface.AddDevices(value.Interface().([]clusterlinkv1alpha1.Device))
	case clusterlinkv1alpha1.RouteName:
		err = e.NetworkInterface.AddRoutes(value.Interface().([]clusterlinkv1alpha1.Route))
	case clusterlinkv1alpha1.IptablesName:
		err = e.NetworkInterface.AddIptables(value.Interface().([]clusterlinkv1alpha1.Iptables))
	case clusterlinkv1alpha1.FdbName:
		err = e.NetworkInterface.AddFdbs(value.Interface().([]clusterlinkv1alpha1.Fdb))
	case clusterlinkv1alpha1.ArpName:
		err = e.NetworkInterface.AddArps(value.Interface().([]clusterlinkv1alpha1.Arp))
	default:
		err = fmt.Errorf("not found this value, name: %s", typeName)
	}
	return err
}

func (e *NetworkManager) WriteSys(configDiff *ConfigDiff) error {
	var errs error
	valueOfDiff := reflect.ValueOf(*configDiff)
	typeOfDiff := reflect.TypeOf(*configDiff)
	for i := 0; i < typeOfDiff.NumField(); i++ {
		valueFieldOfDiff := valueOfDiff.Field(i).Elem()
		typeNameOfDiff := typeOfDiff.Field(i).Name
		if !valueFieldOfDiff.IsZero() {
			config := valueFieldOfDiff.Interface().(clusterlinkv1alpha1.NodeConfigSpec)
			valueOf := reflect.ValueOf(config)
			typeOf := reflect.TypeOf(config)
			for j := 0; j < typeOf.NumField(); j++ {
				valueField := valueOf.Field(j)
				typeName := typeOf.Field(j).Name
				if !valueField.IsZero() {
					var err error
					switch typeNameOfDiff {
					case clusterlinkv1alpha1.DeleteConfigName:
						err = e.delete(valueField, typeName)
					case clusterlinkv1alpha1.CreateConfigName:
						err = e.add(valueField, typeName)
					default:
						errs = fmt.Errorf("not found this value, name: %s", typeNameOfDiff)
						return errs
					}
					if err != nil {
						klog.Warning(err)
						errs = errors.Wrap(err, fmt.Sprint(errs))
					}
				}
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
		DeleteConfig: deleteConfig,
		CreateConfig: createConfig,
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

func (e *NetworkManager) UpdateConfig(cluster *clusterlinkv1alpha1.Cluster) {
	e.NetworkInterface.UpdateCidrConfig(cluster)
}
