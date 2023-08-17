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