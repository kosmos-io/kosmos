package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strings"

	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func FindGlobalNode(nodeName string, globalNodes []v1alpha1.GlobalNode) (*v1alpha1.GlobalNode, bool) {
	for _, globalNode := range globalNodes {
		if globalNode.Name == nodeName {
			return &globalNode, true
		}
	}
	return nil, false
}

func GenerateKubeclient(virtualCluster *v1alpha1.VirtualCluster) (kubernetes.Interface, error) {
	if len(virtualCluster.Spec.Kubeconfig) == 0 {
		return nil, fmt.Errorf("virtualcluster %s kubeconfig is empty", virtualCluster.Name)
	}
	kubeconfigStream, err := base64.StdEncoding.DecodeString(virtualCluster.Spec.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("virtualcluster %s decode target kubernetes kubeconfig %s err: %v", virtualCluster.Name, virtualCluster.Spec.Kubeconfig, err)
	}

	config, err := utils.NewConfigFromBytes(kubeconfigStream)
	if err != nil {
		return nil, fmt.Errorf("generate kubernetes config failed: %s", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("generate K8s basic client failed: %v", err)
	}

	return k8sClient, nil
}

func GetFirstIP(ipNetStrs string) ([]net.IP, error) {
	ipNetStrArray := strings.Split(ipNetStrs, ",")
	if len(ipNetStrArray) > 2 {
		return nil, fmt.Errorf("getFirstIP failed, ipstring is too long: %s", ipNetStrs)
	}

	var ips []net.IP
	for _, ipNetStr := range ipNetStrArray {
		ip, ipNet, err := net.ParseCIDR(ipNetStr)
		if err != nil {
			return nil, fmt.Errorf("parse ipNetStr failed: %s", err)
		}

		networkIP := ip.Mask(ipNet.Mask)

		// IPv4
		if ip.To4() != nil {
			firstIP := make(net.IP, len(networkIP))
			copy(firstIP, networkIP)
			firstIP[len(firstIP)-1]++
			ips = append(ips, firstIP)
			continue
		}

		// IPv6
		firstIP := make(net.IP, len(networkIP))
		copy(firstIP, networkIP)
		for i := len(firstIP) - 1; i >= 0; i-- {
			firstIP[i]++
			if firstIP[i] != 0 {
				break
			}
		}
		ips = append(ips, firstIP)
	}
	return ips, nil
}

func IPV6First(ipNetStr string) (bool, error) {
	ipNetStrArray := strings.Split(ipNetStr, ",")
	if len(ipNetStrArray) > 2 {
		return false, fmt.Errorf("getFirstIP failed, ipstring is too long: %s", ipNetStr)
	}
	return utils.IsIPv6(ipNetStrArray[0]), nil
}

// parseCIDR returns a channel that generates IP addresses in the CIDR range.
func parseCIDR(cidr string) (chan string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	ch := make(chan string)
	go func() {
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			ch <- ip.String()
		}
		close(ch)
	}()
	return ch, nil
}

// inc increments an IP address.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// parseRange returns a channel that generates IP addresses in the range.
func parseRange(ipRange string) (chan string, error) {
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid IP range format: %s", ipRange)
	}
	startIP := net.ParseIP(parts[0])
	endIP := net.ParseIP(parts[1])
	if startIP == nil || endIP == nil {
		return nil, fmt.Errorf("invalid IP address in range: %s", ipRange)
	}

	ch := make(chan string)
	go func() {
		for ip := startIP; !ip.Equal(endIP); inc(ip) {
			ch <- ip.String()
		}
		ch <- endIP.String()
		close(ch)
	}()
	return ch, nil
}

// ParseVIPPool returns a channel that generates IP addresses from the vipPool.
func parseVIPPool(vipPool []string) (chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, entry := range vipPool {
			entry = strings.TrimSpace(entry)
			var ipCh chan string
			var err error
			if strings.Contains(entry, "/") {
				ipCh, err = parseCIDR(entry)
			} else if strings.Contains(entry, "-") {
				ipCh, err = parseRange(entry)
			} else {
				ip := net.ParseIP(entry)
				if ip == nil {
					err = fmt.Errorf("invalid IP address: %s", entry)
				} else {
					ipCh = make(chan string, 1)
					ipCh <- entry
					close(ipCh)
				}
			}
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			for ip := range ipCh {
				ch <- ip
			}
		}
	}()
	return ch, nil
}

// FindAvailableIP finds an available IP address from vipPool that is not in allocatedVips.
func FindAvailableIP(vipPool, allocatedVips []string) (string, error) {
	allocatedSet := make(map[string]struct{})
	for _, ip := range allocatedVips {
		allocatedSet[ip] = struct{}{}
	}

	ipCh, err := parseVIPPool(vipPool)
	if err != nil {
		return "", err
	}

	for ip := range ipCh {
		if _, allocated := allocatedSet[ip]; !allocated {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IP addresses")
}

// Seed the random number generator using crypto/rand
func SecureRandomInt(n int) (int, error) {
	bigN := big.NewInt(int64(n))
	randInt, err := rand.Int(rand.Reader, bigN)
	if err != nil {
		return 0, err
	}
	return int(randInt.Int64()), nil
}

func MapContains(big map[string]string, small map[string]string) bool {
	for k, v := range small {
		if bigV, ok := big[k]; !ok || bigV != v {
			return false
		}
	}
	return true
}
