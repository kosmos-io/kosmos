package net

import (
	"net"

	"k8s.io/klog/v2"
)

// Intersect checks whether the two networks intersect
func Intersect(net1 string, net2 string) bool {
	_, ipNet1, err1 := net.ParseCIDR(net1)
	_, ipNet2, err2 := net.ParseCIDR(net2)

	if err1 != nil || err2 != nil {
		klog.Errorf("the net is invalid, err: %v, %v", err1, err2)
		// In actual scenarios, true is more secure
		return true
	}

	if ipNet1.Contains(ipNet2.IP) || ipNet2.Contains(ipNet1.IP) {
		return true
	}
	return false
}
