package network

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

type NeighType int

const (
	NEIGH_FDB NeighType = iota
	NEIGH_ARP
)

var NEIGH_TYPE_MAP map[NeighType]string = map[NeighType]string{
	NEIGH_ARP: "arp",
	NEIGH_FDB: "fbd",
}

func neighListForArp(linkIndex int) ([]netlink.Neigh, error) {
	var err error
	var neighList []netlink.Neigh
	neighList4, err4 := netlink.NeighList(linkIndex, netlink.FAMILY_V4)
	neighList6, err6 := netlink.NeighList(linkIndex, netlink.FAMILY_V6)
	neighList = append(neighList, neighList4...)
	neighList = append(neighList, neighList6...)
	if err4 != nil || err6 != nil {
		err = fmt.Errorf("list arp err, linkIndex: %d, err4: %s, err6: %s ", linkIndex, err4, err6)
	}
	return neighList, err
}

func getNeighList(linkIndex int, neighType NeighType) ([]netlink.Neigh, error) {
	var err error
	var neighList []netlink.Neigh
	switch neighType {
	case NEIGH_FDB:
		neighList, err = netlink.NeighList(linkIndex, unix.AF_BRIDGE)
	case NEIGH_ARP:
		neighList, err = neighListForArp(linkIndex)
	default:
		return nil, fmt.Errorf("strange neighType %v", neighType)
	}
	return neighList, err
}

func getNeighEntry(dstIP *net.IP, mac *net.HardwareAddr, vxlanIface *netlink.Link, neighType NeighType) *netlink.Neigh {
	switch neighType {
	case NEIGH_FDB:
		return &netlink.Neigh{
			LinkIndex:    (*vxlanIface).Attrs().Index,
			Type:         netlink.NDA_DST,
			Family:       unix.AF_BRIDGE,
			State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
			HardwareAddr: *mac,
			Flags:        netlink.NTF_SELF,
			IP:           *dstIP,
		}
	case NEIGH_ARP:
		family := netlink.FAMILY_V4
		if dstIP.To4() == nil {
			family = netlink.FAMILY_V6
		}
		return &netlink.Neigh{
			LinkIndex:    (*vxlanIface).Attrs().Index,
			Type:         netlink.NDA_DST,
			Family:       family,
			State:        netlink.NUD_PERMANENT,
			HardwareAddr: *mac,
			Flags:        netlink.NTF_SELF,
			IP:           *dstIP,
		}
	default:
		return nil
	}
}

func addEntryToNeigh(dstIP *net.IP, mac *net.HardwareAddr, vxlanIface *netlink.Link, neighType NeighType) error {
	neighObj := getNeighEntry(dstIP, mac, vxlanIface, neighType)

	if neighType == NEIGH_FDB {
		klog.Infof("Try to add fdb")
		if err := netlink.NeighAppend(neighObj); err != nil {
			klog.Errorf("Adding neigh %s (ip %v mac %v) entry,but encountered error: %v", NEIGH_TYPE_MAP[neighType], *dstIP, *mac, err)
			return err
		}
	}
	if neighType == NEIGH_ARP {
		klog.Infof("Try to add arp")
		if err := netlink.NeighSet(neighObj); err != nil {
			klog.Errorf("Adding neigh %s (ip %v mac %v) entry,but encountered error: %v", NEIGH_TYPE_MAP[neighType], *dstIP, *mac, err)
			return err
		}
	}
	return nil
}

func delEntryToNeigh(dstIP *net.IP, mac *net.HardwareAddr, vxlanIface *netlink.Link, neighType NeighType) error {
	neigh := getNeighEntry(dstIP, mac, vxlanIface, neighType)
	if err := netlink.NeighDel(neigh); err != nil {
		klog.Errorf("Deleting neigh %s (ip %v mac %v) entry,but encountered error: %v", NEIGH_TYPE_MAP[neighType], *dstIP, *mac, err)
		return err
	}
	return nil
}

func parseIPMAC(ip string, mac string, devName string) (*net.IP,
	*net.HardwareAddr, *netlink.Link, error) {
	ipAddress := net.ParseIP(ip)
	if ipAddress == nil {
		return nil, nil, nil, fmt.Errorf("ip %s parse error", ip)
	}
	macAddress, err := net.ParseMAC(mac)
	if err != nil {
		klog.Errorf("Mac %s parse error : %v", mac, err)
		return nil, nil, nil, err
	}
	vxlanIface, err := netlink.LinkByName(devName)
	if err != nil {
		klog.Errorf("Can't find interface %s, encounter error : %v", devName, err)
		return nil, nil, nil, err
	}
	return &ipAddress, &macAddress, &vxlanIface, nil
}

func AddNeigh(ip string, mac string, neighType NeighType, devName string) error {
	// No vxlan interface name provides, use DefaultCrossClusterVxLANName
	// as LinkIndex field of Struct Neigh
	klog.Infof("Try to add %s entry, ip %s mac %s", NEIGH_TYPE_MAP[neighType], ip, mac)
	// ToDo: add neigh entry by interface
	ipAddress, macAddress, vxlanIface, err := parseIPMAC(ip, mac, devName)
	if err != nil {
		return err
	}
	err = addEntryToNeigh(ipAddress, macAddress, vxlanIface, neighType)
	if err != nil {
		return err
	}
	klog.Infof("Add %s entry, ip %s mac %s success", NEIGH_TYPE_MAP[neighType], ip, mac)
	return nil
}

func DeleteNeigh(ip string, mac string, neighType NeighType, devName string) error {
	// No vxlan interface name provides, use DefaultCrossClusterVxLANName
	// as the intput parameter LinkIndex field of Struct Neigh
	klog.Infof("Try to delete %s entry, ip %s mac %s", NEIGH_TYPE_MAP[neighType], ip, mac)
	// ToDo: delete neigh entry by interface
	ipAddress, macAddress, vxlanIface, err := parseIPMAC(ip, mac, devName)
	if err != nil {
		return err
	}
	err = delEntryToNeigh(ipAddress, macAddress, vxlanIface, neighType)
	if err != nil {
		return err
	}

	klog.Infof("Delete %s entry, ip %s mac %s success", NEIGH_TYPE_MAP[neighType], ip, mac)
	return nil
}

func DeleteNeighByDevice(dev string, neighType NeighType) error {
	klog.Infof("Try to delete %s entry by interface name %s", NEIGH_TYPE_MAP[neighType], dev)
	vxlanIface, err := netlink.LinkByName(dev)
	if err != nil {
		klog.Errorf("Find interface %s error :%v", dev, err)
		return err
	}
	neighList, err := getNeighList(vxlanIface.Attrs().Index, neighType)
	if err != nil {
		klog.Errorf("Show  %s entries error: %v", NEIGH_TYPE_MAP[neighType], err)
		return err
	}
	if len(neighList) == 0 {
		klog.Info("Thers is no %s entry for interface %s", NEIGH_TYPE_MAP[neighType], dev)
		return nil
	}
	for i := range neighList {
		err := netlink.NeighDel(&neighList[i])
		if err != nil {
			klog.Errorf("Del %s entry error : %v", NEIGH_TYPE_MAP[neighType], err)
			return err
		}
	}
	klog.Infof("Delete %s entry by interface name %s success", NEIGH_TYPE_MAP[neighType], dev)
	return nil
}

func ListNeigh(neighType NeighType) []FDBRecord {
	klog.Infof("Listing all %s entry", NEIGH_TYPE_MAP[neighType])
	// ToDo: list neigh entry by interface
	devs := []string{VXLAN_BRIDGE_NAME, VXLAN_BRIDGE_NAME_6, VXLAN_LOCAL_NAME, VXLAN_LOCAL_NAME_6}
	res := make([]FDBRecord, 0, 5)
	for _, dev := range devs {
		vxlanIface, err := netlink.LinkByName(dev)
		if err != nil {
			continue
		}
		neighList, err := getNeighList(vxlanIface.Attrs().Index, neighType)
		if err != nil {
			klog.Errorf("List vxlan neigh %s entries, but get error : %v", NEIGH_TYPE_MAP[neighType], err)
			continue
		}
		for _, neigh := range neighList {
			if neighType == NEIGH_ARP && neigh.State != netlink.NUD_PERMANENT {
				continue
			}
			tempNeighEntry := FDBRecord{
				IP:  neigh.IP.String(),
				Mac: neigh.HardwareAddr.String(),
				Dev: dev,
			}
			res = append(res, tempNeighEntry)
		}
	}
	if len(res) == 0 {
		klog.Infof("There is no %s entry", NEIGH_TYPE_MAP[neighType])
		return nil
	}
	return res
}
