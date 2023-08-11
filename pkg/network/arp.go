package network

import (
	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func AddARP(ip string, mac string, devName string) error {
	return AddNeigh(ip, mac, NEIGH_ARP, devName)
}

func DeleteARP(ip string, mac string, devName string) error {
	return DeleteNeigh(ip, mac, NEIGH_ARP, devName)
}

func DeleteARPByDevice(dev string) error {
	return DeleteNeighByDevice(dev, NEIGH_ARP)
}

func ListARP() []FDBRecord {
	return ListNeigh(NEIGH_ARP)
}

func loadArps() ([]clusterlinkv1alpha1.Arp, error) {
	ret := []clusterlinkv1alpha1.Arp{}

	arps := ListARP()
	for _, arp := range arps {
		ret = append(ret, clusterlinkv1alpha1.Arp{
			IP:  arp.IP,
			Mac: arp.Mac,
			Dev: arp.Dev,
		})
	}
	return ret, nil
}

func deleteArp(arp clusterlinkv1alpha1.Arp) error {
	return DeleteARP(arp.IP, arp.Mac, arp.Dev)
}

func addArp(arp clusterlinkv1alpha1.Arp) error {
	return AddARP(arp.IP, arp.Mac, arp.Dev)
}
