package network

import (
	"net"
	"os"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
)

func newTableRule(tableID, family int) *netlink.Rule {
	rule := netlink.NewRule()
	rule.Table = tableID
	rule.Priority = tableID
	rule.Family = family

	return rule
}

func ruleAddIfNotPresent(rule *netlink.Rule) error {
	err := netlink.RuleAdd(rule)
	if err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "failed to add rule %s", rule)
	}

	return nil
}

func routeList(link netlink.Link, family int) ([]netlink.Route, error) {
	var routeFilter *netlink.Route
	if link != nil {
		routeFilter = &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Table:     TABLE_ID,
		}
	}
	return netlink.RouteListFiltered(family, routeFilter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
}

func getRoutes(dev string) ([]netlink.Route, error) {
	device, err := netlink.LinkByName(dev)
	if err != nil {
		// klog.Errorf("Get device by name but encountered an error : %v", err)
		return nil, err
	}

	currentRouteList, err := routeList(device, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}

	ret := []netlink.Route{}

	for _, route := range currentRouteList {
		if route.Table == TABLE_ID {
			ret = append(ret, route)
		}
	}

	// ret = append(ret, currentRouteList...)

	return ret, nil
}

func loadRoutes() ([]clusterlinkv1alpha1.Route, error) {
	ret := []clusterlinkv1alpha1.Route{}

	for _, d := range ALL_DEVICES {
		routes, err := getRoutes(d.name)
		if err != nil {
			if errors.As(err, &netlink.LinkNotFoundError{}) {
				continue
			} else {
				return nil, err
			}
		}
		for _, r := range routes {
			ret = append(ret, clusterlinkv1alpha1.Route{
				CIDR: r.Dst.String(),
				Gw:   r.Gw.String(),
				Dev:  d.name,
			})
		}
	}

	return ret, nil
}

func addRoute(r clusterlinkv1alpha1.Route) error {

	ipAddressList := []string{r.CIDR}
	gw := r.Gw
	dev := r.Dev

	if len(ipAddressList) == 0 {
		return nil
	}
	device, err := netlink.LinkByName(dev)
	if err != nil {
		klog.Errorf("Get device by name but encountered an error : %v", err)
		return err
	}
	klog.Infof("Try to add routes")
	for _, ipAddress := range ipAddressList {

		_, ipNetMask, err := net.ParseCIDR(ipAddress)

		if err != nil {
			return errors.Wrapf(err, "ParseCIDR %s,but encountered some error: %v", ipAddress, err)
		}

		route := &netlink.Route{
			Dst: &net.IPNet{
				IP:   ipNetMask.IP,
				Mask: ipNetMask.Mask,
			},
			LinkIndex: device.Attrs().Index,
			Gw:        net.ParseIP(gw),
			Scope:     netlink.SCOPE_UNIVERSE,
			Type:      netlink.NDA_DST,
			Flags:     int(netlink.FLAG_ONLINK),
			Table:     TABLE_ID,
		}
		klog.Infof("Try to add route through dev %s (net hop %s) to %s (mask:%v)",
			dev, net.ParseIP(gw).String(), ipNetMask.IP.String(), ipNetMask.Mask)
		err = netlink.RouteAdd(route)
		if err != nil {
			klog.Infof("Try to add route but encountered an error : %v, route: %v", err, route)
			return err
		}
	}
	klog.Infof("Add routes success")
	return nil
}

func deleteRoute(r clusterlinkv1alpha1.Route) error {

	ipAddressList := []string{r.CIDR}
	gw := r.Gw
	dev := r.Dev

	if len(ipAddressList) == 0 {
		return nil
	}
	device, err := netlink.LinkByName(dev)
	if err != nil {
		klog.Errorf("Get device by name but encountered an error : %v", err)
		return err
	}
	klog.Info("Try to delete routes")
	for _, ipAddress := range ipAddressList {

		_, ipNetMask, err := net.ParseCIDR(ipAddress)

		if err != nil {
			return errors.Wrapf(err, "ParseCIDR %s,but encountered some error: %v", ipAddress, err)
		}

		route := &netlink.Route{
			Dst: &net.IPNet{
				IP:   ipNetMask.IP,
				Mask: ipNetMask.Mask,
			},
			LinkIndex: device.Attrs().Index,
			Gw:        net.ParseIP(gw),
			Scope:     netlink.SCOPE_UNIVERSE,
			Type:      netlink.NDA_DST,
			Flags:     int(netlink.FLAG_ONLINK),
			Table:     TABLE_ID,
		}
		err = netlink.RouteDel(route)
		if err != nil {
			klog.Errorf("Try to delete route but encountered an error : %v", err)
			return err
		}
	}
	klog.Info("Delete routes success")
	return nil
}
