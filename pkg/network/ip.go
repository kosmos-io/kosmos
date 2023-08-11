package network

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

func getFristScopeIPInLink(vxlanIface netlink.Link, name string, familyType int) (string, error) {
	addrList, err := netlink.AddrList(vxlanIface, familyType)
	if err != nil {
		return "", err
	}
	if len(addrList) == 0 {
		klog.Errorf("Device %s has none ip addr, familyType: %v", name, familyType)
		return "", fmt.Errorf("device %s has none ip addr, familyType: %v", name, familyType)
	}
	for _, addr := range addrList {
		if addr.Scope == unix.RT_SCOPE_UNIVERSE {
			return addr.IP.String(), nil
		}
	}
	return "", fmt.Errorf("there is no scop ip for dev: %v", name)
}

func getFirstGlobalIPByDevName(name string) ([]string, error) {
	vxlanIface, err := netlink.LinkByName(name)
	if err != nil {
		klog.Errorf("Try to find device by name %s, get error : %v", name, err)
		return nil, err
	}
	ipv4, err := getFristScopeIPInLink(vxlanIface, name, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("Try to retrieve addr v4 list of device %s, get error: %v", name, err)
		return nil, err
	}
	ipv6, err := getFristScopeIPInLink(vxlanIface, name, netlink.FAMILY_V6)
	if err != nil {
		klog.Errorf("Try to retrieve addr v6 list of device %s, get error: %v", name, err)
		return nil, err
	}

	return []string{ipv4, ipv6}, nil
}
