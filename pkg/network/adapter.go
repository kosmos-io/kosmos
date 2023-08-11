package network

import (
	"fmt"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/pkg/errors"
)

type DefaultNetWork struct {
}

func (n *DefaultNetWork) LoadSysConfig() (*clusterlinkv1alpha1.NodeConfigSpec, error) {
	var errs error
	nodeConfigSpec := &clusterlinkv1alpha1.NodeConfigSpec{}

	devices, err := loadDevices()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.Devices = devices
	}

	routes, err := loadRoutes()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.Routes = routes
	}

	iptables, err := loadIptables()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.Iptables = iptables
	}

	fdbs, err := loadFdbs()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.Fdbs = fdbs
	}

	arps, err := loadArps()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.Arps = arps
	}

	return nodeConfigSpec, errs
}

func (n *DefaultNetWork) DeleteArps(arps []clusterlinkv1alpha1.Arp) error {
	var errs error
	for _, arp := range arps {
		if err := deleteArp(arp); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete arp error, fdb: %v", arp))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteFdbs(fdbs []clusterlinkv1alpha1.Fdb) error {
	var errs error
	for _, fdb := range fdbs {
		if err := deleteFdb(fdb); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete fdb error, fdb: %v", fdb))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteIptables(records []clusterlinkv1alpha1.Iptables) error {
	var errs error
	for _, iptable := range records {
		if err := deleteIptable(iptable); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete iptable error, deivce name: %v", iptable))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteRoutes(routes []clusterlinkv1alpha1.Route) error {
	var errs error
	for _, route := range routes {
		if err := deleteRoute(route); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete route error, deivce name: %v", route))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteDevices(devices []clusterlinkv1alpha1.Device) error {
	var errs error
	for _, device := range devices {
		if err := deleteDevice(device); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete device error, deivce name: %s", device.Name))
		}
	}
	return errs
}

func (n *DefaultNetWork) UpdateArps([]clusterlinkv1alpha1.Arp) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateFdbs(fdbs []clusterlinkv1alpha1.Fdb) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateIptables([]clusterlinkv1alpha1.Iptables) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateRoutes([]clusterlinkv1alpha1.Route) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateDevices([]clusterlinkv1alpha1.Device) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) AddArps(arps []clusterlinkv1alpha1.Arp) error {
	var errs error
	for _, arp := range arps {
		if err := addArp(arp); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("create arp error : %v", arp))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddFdbs(fdbs []clusterlinkv1alpha1.Fdb) error {
	var errs error
	for _, fdb := range fdbs {
		if err := addFdb(fdb); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("create fdb error, deivce name: %v", fdb))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddIptables(iptabless []clusterlinkv1alpha1.Iptables) error {
	var errs error
	for _, ipts := range iptabless {
		if err := addIptables(ipts); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("create iptable error, deivce name: %v", ipts))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddRoutes(routes []clusterlinkv1alpha1.Route) error {
	var errs error
	for _, route := range routes {
		if err := addRoute(route); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("create route error, deivce name: %v", route))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddDevices(devices []clusterlinkv1alpha1.Device) error {
	var errs error
	for _, device := range devices {
		if err := addDevice(device); err == nil {
			errs = errors.Wrap(err, fmt.Sprintf("create device error, deivce name: %s", device.Name))
		}
	}
	return errs
}
