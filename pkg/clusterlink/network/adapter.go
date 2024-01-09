package network

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
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

	xfrmpolicies, err := loadXfrmPolicy()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.XfrmPolicies = xfrmpolicies
	}

	xfrmstates, err := loadXfrmState()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.XfrmStates = xfrmstates
	}

	ipsets, err := loadIPsetAvoidMasq()
	if err != nil {
		errs = errors.Wrap(err, fmt.Sprint(errs))
	} else {
		nodeConfigSpec.IPsetsAvoidMasqs = ipsets
	}

	return nodeConfigSpec, errs
}

func (n *DefaultNetWork) DeleteArps(arps []clusterlinkv1alpha1.Arp) error {
	var errs error
	for _, arp := range arps {
		if err := deleteArp(arp); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete arp error, fdb: %v", arp))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteFdbs(fdbs []clusterlinkv1alpha1.Fdb) error {
	var errs error
	for _, fdb := range fdbs {
		if err := deleteFdb(fdb); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete fdb error, fdb: %v", fdb))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteIptables(records []clusterlinkv1alpha1.Iptables) error {
	var errs error
	for _, iptable := range records {
		if err := deleteIptable(iptable); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete iptable error, deivce name: %v", iptable))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteRoutes(routes []clusterlinkv1alpha1.Route) error {
	var errs error
	for _, route := range routes {
		if err := deleteRoute(route); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete route error, deivce name: %v", route))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteDevices(devices []clusterlinkv1alpha1.Device) error {
	var errs error
	for _, device := range devices {
		if err := deleteDevice(device); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("delete device error, deivce name: %s", device.Name))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteIPsetsAvoidMasq(ipsets []clusterlinkv1alpha1.IPset) error {
	var errs error
	for _, ipset := range ipsets {
		err := deleteIPset(ipset)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("add ipset avoid masq error: %v", ipset))
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

func (n *DefaultNetWork) UpdateXfrmPolicies([]clusterlinkv1alpha1.XfrmPolicy) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateXfrmStates([]clusterlinkv1alpha1.XfrmState) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) UpdateIPsetsAvoidMasq([]clusterlinkv1alpha1.IPset) error {
	return ErrNotImplemented
}

func (n *DefaultNetWork) AddArps(arps []clusterlinkv1alpha1.Arp) error {
	var errs error
	for _, arp := range arps {
		if err := addArp(arp); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("create arp error : %v", arp))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddFdbs(fdbs []clusterlinkv1alpha1.Fdb) error {
	var errs error
	for _, fdb := range fdbs {
		if err := addFdb(fdb); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("create fdb error, deivce name: %v", fdb))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddIptables(iptabless []clusterlinkv1alpha1.Iptables) error {
	var errs error
	for _, ipts := range iptabless {
		if err := addIptables(ipts); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("create iptable error, deivce name: %v", ipts))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddRoutes(routes []clusterlinkv1alpha1.Route) error {
	var errs error
	for _, route := range routes {
		if err := addRoute(route); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("create route error, deivce name: %v", route))
		}
	}
	return errs
}

// For reference:
// https://github.com/flannel-io/flannel
func (n *DefaultNetWork) AddXfrmPolicies(xfrmpolicies []clusterlinkv1alpha1.XfrmPolicy) error {
	var errs error
	for _, xfrmpolicy := range xfrmpolicies {
		srcIP := net.ParseIP(xfrmpolicy.LeftIP)
		dstIP := net.ParseIP(xfrmpolicy.RightIP)
		_, srcNet, _ := net.ParseCIDR(xfrmpolicy.LeftNet)
		_, dstNet, _ := net.ParseCIDR(xfrmpolicy.RightNet)
		reqID := xfrmpolicy.ReqID

		var err error
		var xfrmpolicydir netlink.Dir
		switch v1alpha1.IPSECDirection(xfrmpolicy.Dir) {
		case v1alpha1.IPSECOut:
			xfrmpolicydir = netlink.XFRM_DIR_OUT
		case v1alpha1.IPSECIn:
			xfrmpolicydir = netlink.XFRM_DIR_IN
		case v1alpha1.IPSECFwd:
			xfrmpolicydir = netlink.XFRM_DIR_FWD
		}
		err = AddXFRMPolicy(srcNet, dstNet, srcIP, dstIP, xfrmpolicydir, reqID)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("error adding ipsec policy: %v", xfrmpolicy))
		}
	}
	return errs
}

// For reference:
// https://github.com/flannel-io/flannel
func (n *DefaultNetWork) DeleteXfrmPolicies(xfrmpolicies []clusterlinkv1alpha1.XfrmPolicy) error {
	var errs error
	for _, xfrmpolicy := range xfrmpolicies {
		srcIP := net.ParseIP(xfrmpolicy.LeftIP)
		dstIP := net.ParseIP(xfrmpolicy.RightIP)
		_, srcNet, _ := net.ParseCIDR(xfrmpolicy.LeftNet)
		_, dstNet, _ := net.ParseCIDR(xfrmpolicy.RightNet)
		reqID := xfrmpolicy.ReqID

		var xfrmpolicydir netlink.Dir
		switch v1alpha1.IPSECDirection(xfrmpolicy.Dir) {
		case v1alpha1.IPSECOut:
			xfrmpolicydir = netlink.XFRM_DIR_OUT
		case v1alpha1.IPSECIn:
			xfrmpolicydir = netlink.XFRM_DIR_IN
		case v1alpha1.IPSECFwd:
			xfrmpolicydir = netlink.XFRM_DIR_FWD
		}

		if reqID != v1alpha1.DefaultReqID {
			klog.Info("Xfrm policy %v reqID is %d, not created by kosmos", xfrmpolicy, reqID)
			continue
		}
		err := DeleteXFRMPolicy(srcNet, dstNet, srcIP, dstIP, xfrmpolicydir, reqID)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("error deleting ipsec policy: %v", xfrmpolicy))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddXfrmStates(xfrmstates []clusterlinkv1alpha1.XfrmState) error {
	var errs error
	for _, xfrmstate := range xfrmstates {
		srcIP := net.ParseIP(xfrmstate.LeftIP)
		dstIP := net.ParseIP(xfrmstate.RightIP)
		reqID := xfrmstate.ReqID
		err := AddXFRMState(srcIP, dstIP, reqID, int(xfrmstate.SPI), xfrmstate.PSK)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("error adding ipsec state: %v", xfrmstate))
		}
	}
	return errs
}

func (n *DefaultNetWork) DeleteXfrmStates(xfrmstates []clusterlinkv1alpha1.XfrmState) error {
	var errs error
	for _, xfrmstate := range xfrmstates {
		srcIP := net.ParseIP(xfrmstate.LeftIP)
		dstIP := net.ParseIP(xfrmstate.RightIP)
		reqID := xfrmstate.ReqID
		if reqID != v1alpha1.DefaultReqID {
			klog.Info("Xfrm state %v reqID is %d, not created by kosmos", xfrmstate, reqID)
			continue
		}
		err := DeleteXFRMState(srcIP, dstIP, reqID, int(xfrmstate.SPI), xfrmstate.PSK)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("error deleting ipsec state: %v", xfrmstate))
		}
	}
	return errs
}

func (n *DefaultNetWork) AddIPsetsAvoidMasq(ipsets []clusterlinkv1alpha1.IPset) error {
	var errs error
	for _, ipset := range ipsets {
		err := addIPset(ipset)
		if err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("add ipset avoid masq error: %v", ipset))
		}
	}
	if len(ipsets) > 0 {
		err := ensureAvoidMasqRule()
		if err != nil {
			errs = errors.Wrap(err, "create iptables rule to avoid masq,error")
		}
	}
	return errs
}

func (n *DefaultNetWork) AddDevices(devices []clusterlinkv1alpha1.Device) error {
	var errs error
	for _, device := range devices {
		if err := addDevice(device); err != nil {
			errs = errors.Wrap(err, fmt.Sprintf("create device error, deivce name: %s", device.Name))
		}
	}
	return errs
}

func (n *DefaultNetWork) InitSys() {
	if err := CreateGlobalNetIptablesChains(); err != nil {
		klog.Warning(err)
	}

	if err := EnableLooseModeForFlannel(); err != nil {
		klog.Warning(err)
	}
}

func (n *DefaultNetWork) UpdateCidrConfig(cluster *clusterlinkv1alpha1.Cluster) {
	UpdateCidr(cluster.Spec.ClusterLinkOptions.BridgeCIDRs.IP, cluster.Spec.ClusterLinkOptions.BridgeCIDRs.IP6, cluster.Spec.ClusterLinkOptions.LocalCIDRs.IP, cluster.Spec.ClusterLinkOptions.LocalCIDRs.IP6)
}
