package network

import (
	"fmt"
	"net"
	"syscall"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/utils"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

type IfaceInfo struct {
	MTU  int
	name string
	// index int
	ip  string
	ip6 string
}

func getIfaceIPByName(name string) (*IfaceInfo, error) {
	// TODO: add cache
	// netlink.L
	devIface := &IfaceInfo{}
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "When we get device by linkinx, get error : %v, name: %v", err, name)
	}

	devIface.MTU = iface.Attrs().MTU
	devIface.name = iface.Attrs().Name

	addrListV4, err := getFristScopeIPInLink(iface, name, netlink.FAMILY_V4)
	if err == nil {
		devIface.ip = addrListV4
	} else {
		klog.Infof("Try to retrieve addr v4 list of device %s, get error: %v", name, err)
	}

	addrListV6, err := getFristScopeIPInLink(iface, name, netlink.FAMILY_V6)
	if err == nil {
		devIface.ip6 = addrListV6
	} else {
		klog.Infof("Try to retrieve addr v6 list of device %s, get error: %v", name, err)
	}
	return devIface, nil
}

func createNewVxlanIface(name string, addrIPWithMask *netlink.Addr, vxlanId int, vxlanPort int, hardwareAddr net.HardwareAddr, rIface *IfaceInfo, deviceIP string) error {
	// srcAddr := rIface.ip

	klog.Infof("name %v  ------------------------- %v", name, deviceIP)
	iface := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:         name,
			MTU:          rIface.MTU - vxlanOverhead,
			Flags:        net.FlagUp,
			HardwareAddr: hardwareAddr,
		},
		SrcAddr:  net.ParseIP(deviceIP),
		VxlanId:  vxlanId,
		Port:     vxlanPort,
		Learning: false,
	}

	err := netlink.LinkAdd(iface)
	if err != nil {
		if errors.Is(err, syscall.EEXIST) {
			_, err := netlink.LinkByName(iface.LinkAttrs.Name)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to retrieve link info, name: %v", iface.LinkAttrs.Name))
			}
		}
		return err
	}
	// create rule table
	family := netlink.FAMILY_V4
	if utils.IsIPv6(deviceIP) {
		family = netlink.FAMILY_V6
	}
	err = ruleAddIfNotPresent(newTableRule(TABLE_ID, family))
	if err != nil {
		klog.Errorf("Add table rule %v , get error : %v", addrIPWithMask, err)
		return err
	}

	klog.Infof("name %v  ------------------------- addrIPWithMask %v", name, addrIPWithMask)

	err = netlink.AddrAdd(iface, addrIPWithMask)
	if err != nil {
		klog.Errorf("Add address %v to vxlan interface,get error : %v", addrIPWithMask, err)
		return err
	}

	return nil
}

func CIDRIPGenerator(ipcidr string, internalIP string) (*net.IP, error) {
	cidrip, ipNet, err := net.ParseCIDR(ipcidr)
	if err != nil {
		return nil, fmt.Errorf("CIDRIPGenerator err: %v", err)
	}

	nodeIP := net.ParseIP(internalIP)

	ret := net.ParseIP("0.0.0.0")
	for i := range ipNet.Mask {
		ret[len(ret)-i-1] = ^byte(ipNet.Mask[len(ipNet.Mask)-i-1])
	}

	for i := range nodeIP {
		ret[i] = byte(ret[i]) & byte(nodeIP[i])
	}

	for i := range cidrip {
		ret[i] = byte(ret[i]) | byte(cidrip[i])
	}

	return &ret, nil
}

// load device info from environment
func loadDevices() ([]clusterlinkv1alpha1.Device, error) {
	ret := []clusterlinkv1alpha1.Device{}

	for _, d := range ALL_DEVICES {
		vxlanIface, err := netlink.LinkByName(d.name)
		if err != nil {
			if errors.As(err, &netlink.LinkNotFoundError{}) {
				continue
			} else {
				return nil, err
			}
		}

		if vxlanIface.Type() != (&netlink.Vxlan{}).Type() {
			return nil, fmt.Errorf("device name: %s is not vxlan", d.name)
		}
		vxlan := vxlanIface.(*netlink.Vxlan)

		addrList, err := netlink.AddrList(vxlanIface, d.family)

		if err != nil {
			return nil, err
		}

		var vxlanNet *net.IPNet

		for _, addr := range addrList {
			if addr.Scope == unix.RT_SCOPE_UNIVERSE {
				vxlanNet = addr.IPNet
				break
			}
		}

		routes, err := netlink.RouteList(nil, d.family)

		if err != nil {
			return nil, err
		}

		interfaceIndex := -1
		vxlanIPAddr := vxlanNet.IP.String()

		for _, r := range routes {
			internalIP := r.Src.String()
			if ip, err := CIDRIPGenerator(d.cidr, internalIP); err == nil {
				if ip.String() == vxlanIPAddr {
					interfaceIndex = r.LinkIndex
					break
				}
			}
		}

		if interfaceIndex == -1 {
			klog.Warning("can not find the dev for vxlan, name: %s", d.name)
			continue
		}

		defaultIface, err := netlink.LinkByIndex(interfaceIndex)
		if err != nil {
			klog.Errorf("When we get device by linkinx, get error : %v", err)
			return nil, err
		}

		ret = append(ret, clusterlinkv1alpha1.Device{
			Type:    clusterlinkv1alpha1.DeviceType(vxlanIface.Type()),
			Name:    vxlan.LinkAttrs.Name,
			Addr:    vxlanNet.String(),
			Mac:     vxlan.LinkAttrs.HardwareAddr.String(),
			BindDev: defaultIface.Attrs().Name,
			ID:      int32(vxlan.VxlanId),
			Port:    int32(vxlan.Port),
		})
	}

	return ret, nil
}

func addDevice(d clusterlinkv1alpha1.Device) error {
	cidrip, ipNet, err := net.ParseCIDR(d.Addr)

	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("parse device cidr failed, cidr: %s", d.Addr))
	}

	addrIPvWithMask := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   cidrip,
			Mask: ipNet.Mask,
		},
	}

	currentIfaceInfo, err := getIfaceIPByName(d.BindDev)
	if err != nil {
		return errors.Wrap(err, "add device failed")
	}

	id := int(d.ID)

	port := int(d.Port)

	hardwareAddr, err := net.ParseMAC(d.Mac)
	if err != nil {
		return errors.Wrap(err, "add device failed when translate mac")
	}

	deviceIP := currentIfaceInfo.ip
	family := netlink.FAMILY_V4
	if utils.IsIPv6(cidrip.String()) {
		deviceIP = currentIfaceInfo.ip6
		family = netlink.FAMILY_V6
	}

	if err = createNewVxlanIface(d.Name, addrIPvWithMask, id, port, hardwareAddr, currentIfaceInfo, deviceIP); err != nil {
		klog.Errorf("ipv4 err: %v", err)
		return err
	}
	klog.Infof("add  vxlan %v  %v", d.Name, deviceIP)

	if err := UpdateDefaultIptablesAndKernalConfig(d.BindDev, family); err != nil {
		return err
	}

	if err := updateDeviceConfig(d.Name, family); err != nil {
		return err
	}

	return nil
}

func deleteDevice(d clusterlinkv1alpha1.Device) error {
	klog.Infof("Try to delete vxlan interface %s", d.Name)
	ifaceToDel, err := netlink.LinkByName(d.Name)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			klog.Warning("the Vxlan to be deleted does not exist %s", d.Name)
			return nil
		}
		return errors.Wrap(err, fmt.Sprintf("can't find vxlan interface %s to del, because encountered an error : %v", d.Name, err))
	}
	err = netlink.LinkDel(ifaceToDel)
	if err != nil {
		klog.Errorf("Try to del device %s but encountered an error : %v", d.Name, err)
		return err
	}
	klog.Infof("Delete vxlan interface %s success", d.Name)
	return nil
}

func updateDeviceConfig(name string, ipFamily int) error {
	if ipFamily == netlink.FAMILY_V6 {
		if err := UpdateDefaultIp6tablesBehavior(name); err != nil {
			return err
		}
		if err := EnableDisableIPV6ByIFaceNmae(name); err != nil {
			return err
		}
	} else {
		if err := UpdateDefaultIp4tablesBehavior(name); err != nil {
			return err
		}

		if err := EnableLooseModeByIFaceNmae(name); err != nil {
			return err
		}
	}
	return nil

}

func UpdateDefaultIptablesAndKernalConfig(name string, ipFamily int) error {
	klog.Infof("LoadRouteInface ipFamily: %v", ipFamily)

	// ipv6
	if ipFamily == netlink.FAMILY_V6 {
		if err := UpdateDefaultIp6tablesBehavior(name); err != nil {
			return err
		}
		if err := EnableDisableIPV6ByIFaceNmae(name); err != nil {
			return err
		}
	}

	if ipFamily == netlink.FAMILY_V4 {
		if err := UpdateDefaultIp4tablesBehavior(name); err != nil {
			return err
		}
		if err := EnableLooseModeByIFaceNmae(name); err != nil {
			return err
		}

		// tunl0 device
		if err := UpdateDefaultIp4tablesBehavior("tunl0"); err != nil {
			klog.Errorf("Try to add iptables rule for tunl0: %v", err)
		}

		// tunl0 device
		if err := EnableLooseModeByIFaceNmae("tunl0"); err != nil {
			klog.Errorf("Try to change kernel parameters(rp_filter) for tunl0: %v", err)
		}
	}

	klog.Infof("Get default interface %s, mtu %d, ipv4 %s, ipv6: %s", name)

	return nil
}
