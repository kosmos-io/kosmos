package helpers

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	constants "github.com/kosmos.io/clusterlink/pkg/network"
)

type IPType int

const (
	IPV4 IPType = iota
	IPV6
)

func GetIPType(s string) IPType {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return IPV4
		case ':':
			return IPV6
		}
	}
	return -1
}

func GenerateMac() net.HardwareAddr {
	buf := make([]byte, 6)
	var mac net.HardwareAddr

	_, err := rand.Read(buf)
	if err != nil {
	}

	// Set the local bit
	buf[0] = (buf[0] | 2) & 0xfe

	mac = append(mac, buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
	return mac
}

func GenerateVxlanIP(underlayIP string, destNetString string) (string, error) {
	if GetIPType(underlayIP) != GetIPType(destNetString) {
		return "", fmt.Errorf("GenerateVxLocalIP, different ip types: %s, %s", underlayIP, destNetString)
	}

	ip := net.ParseIP(underlayIP)
	if ip == nil {
		return "", fmt.Errorf("GenerateVxLocalIP, parse underlay ip: %s error", underlayIP)
	}

	_, destNet, err := net.ParseCIDR(destNetString)
	if err != nil {
		return "", fmt.Errorf("GenerateVxLocalIP, parse destNetString: %s with error %v", destNetString, err)
	}

	var changeIPNet func(ip net.IP, destNet net.IPNet) (net.IPNet, error)
	if GetIPType(underlayIP) == IPV4 {
		changeIPNet = changeIPNetIPV4
	} else {
		changeIPNet = changeIPNetIPV6
	}

	newIP, err := changeIPNet(ip, *destNet)
	if err != nil {
		return "", err
	}

	return newIP.String(), nil
}

func changeIPNetIPV4(ip net.IP, destNet net.IPNet) (net.IPNet, error) {

	ipBytes := ip.To4()
	destNetBytes := destNet.IP.To4()
	maskSize, _ := destNet.Mask.Size()

	ipBits := binary.BigEndian.Uint32(ipBytes)
	destNetBits := binary.BigEndian.Uint32(destNetBytes)

	v := ((destNetBits >> (32 - maskSize)) << (32 - maskSize)) | ((ipBits << maskSize) >> maskSize)

	newIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(newIP, v)

	return net.IPNet{
		IP:   newIP,
		Mask: destNet.Mask,
	}, nil
}

func changeIPNetIPV6(ip net.IP, destNet net.IPNet) (net.IPNet, error) {

	ipBytes := []byte(ip)
	maskBytes := []byte(destNet.Mask)
	destIPBytes := []byte(destNet.IP)

	targetIP := make(net.IP, len(ipBytes))

	for k, _ := range ipBytes {
		invertedMask := maskBytes[k] ^ 0xff
		targetIP[k] = (invertedMask & ipBytes[k]) | (destIPBytes[k] & maskBytes[k])
	}

	return net.IPNet{
		IP:   targetIP,
		Mask: destNet.Mask,
	}, nil
}

func BuildVxlanDevice(devName string, underlayIP string, destNetString string, bindDev string) *v1alpha1.Device {
	var dev *v1alpha1.Device
	if devName == constants.VXLAN_BRIDGE_NAME {
		dev = &v1alpha1.Device{
			Name: constants.VXLAN_BRIDGE_NAME,
			ID:   constants.VXLAN_BRIDGE_ID,
			Port: constants.VXLAN_BRIDGE_PORT,
		}
	} else if devName == constants.VXLAN_BRIDGE_NAME_6 {
		dev = &v1alpha1.Device{
			Name: constants.VXLAN_BRIDGE_NAME_6,
			ID:   constants.VXLAN_BRIDGE_ID_6,
			Port: constants.VXLAN_BRIDGE_PORT_6,
		}
	} else if devName == constants.VXLAN_LOCAL_NAME {
		dev = &v1alpha1.Device{
			Name: constants.VXLAN_LOCAL_NAME,
			ID:   constants.VXLAN_LOCAL_ID,
			Port: constants.VXLAN_LOCAL_PORT,
		}
	} else if devName == constants.VXLAN_LOCAL_NAME_6 {
		dev = &v1alpha1.Device{
			Name: constants.VXLAN_LOCAL_NAME_6,
			ID:   constants.VXLAN_LOCAL_ID_6,
			Port: constants.VXLAN_LOCAL_PORT_6,
		}
	} else {
		klog.Errorf("buildVxlanDevice, unknown vxlan name %s", devName)
		return nil
	}

	addr, err := GenerateVxlanIP(underlayIP, destNetString)
	if err != nil {
		klog.Errorf("failed to create %s, err: %v", dev.Name, err)
		return nil
	}

	dev.Addr = addr
	dev.Mac = GenerateMac().String()
	dev.BindDev = bindDev
	dev.Type = v1alpha1.VxlanDevice

	return dev
}
