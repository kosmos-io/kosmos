package ipam

import (
	"fmt"
	"net"
	"sync"
)

func hasUsed(strs []net.IP, t net.IP) bool {
	for _, u := range strs {
		if u.String() == t.String() {
			return true
		}
	}
	return false
}

func allocateOne(used []net.IP, cidr string) (net.IP, net.IPMask, error) {
	_, ipNet, err := net.ParseCIDR(cidr)

	if err != nil {
		return nil, nil, err
	}

	i := make([]byte, len(ipNet.IP))
	copy(i, ipNet.IP)

	for ipNet.Contains(i) {
		if !hasUsed(used, i) {
			l := len(i)
			// don't count net and broadcast
			if i[l-1] != 0x00 && i[l-1] != 0xff {
				return i, ipNet.Mask, nil
			}
		}
		if err := increment(i); err != nil {
			return nil, nil, err
		}
	}

	return nil, nil, fmt.Errorf("allocate ip error %s", net.IP(i).String())
}

func increment(a net.IP) error {
	i := len(a) - 1
	for ; i > 0; i-- {
		if a[i] == 0xff {
			a[i] = 0x00
			continue
		}
		a[i] = a[i] + 1
		break
	}
	if i <= 0 {
		return fmt.Errorf("ip increment error")
	}
	return nil
}

type IPPool struct {
	sync.Mutex
	cidr  string
	cidr6 string
	pool  map[string]net.IP // [clusterName-nodeName]: [IP]
}

func (i *IPPool) Release(key string) {
	i.Lock()
	defer i.Unlock()
	delete(i.pool, key)
}

func (i *IPPool) Allocate(key string) (net.IP, int, int, error) {
	i.Lock()
	defer i.Unlock()
	if i.pool == nil {
		i.pool = map[string]net.IP{}
	}

	v := i.pool[key]

	if len(v) != 0 {
		return v, 0, 0, nil
	}

	values := make([]net.IP, 0, len(i.pool))

	for k := range i.pool {
		values = append(values, i.pool[k])
	}

	newip, mask, err := allocateOne(values, i.cidr)
	if err != nil {
		return nil, 0, 0, err
	}

	i.pool[key] = newip
	ones, bits := mask.Size()

	return newip, ones, bits, nil
}

func (i *IPPool) ToIPv6(ipv4 net.IP) (net.IP, error) {
	ip6, _, err := net.ParseCIDR(i.cidr6)
	if err != nil {
		return nil, err
	}
	s := 0
	if len(ipv4) == 16 {
		s = 12
	}
	v := append(ip6[:12], ipv4[s:]...)

	return v, nil
}

func (i *IPPool) Print() {
	for k, v := range i.pool {
		fmt.Printf("%s: %s", k, v.String())
		fmt.Println("")
	}
}

func NewIPPool(cidr string, cidr6 string, pool map[string]net.IP) *IPPool {
	return &IPPool{
		pool:  pool,
		cidr:  cidr,
		cidr6: cidr6,
	}
}
