package network

import (
	"fmt"
	"math/bits"
	"net"
	"syscall"

	ipt "github.com/coreos/go-iptables/iptables"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"k8s.io/kubernetes/pkg/util/ipset"
	proxyipset "k8s.io/kubernetes/pkg/util/ipset"
	utilexec "k8s.io/utils/exec"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	nmhelpers "github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/helpers"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network/iptables"
)

const (
	KosmosIPsetVoidMasq = "kosmosipset"
)

func CaculateMaskSize(ipnet net.IPNet) uint8 {
	var maskSize uint8 = 0
	for _, maskbyte := range []byte(ipnet.Mask) {
		maskSize += uint8(bits.OnesCount8(maskbyte))
	}
	return maskSize
}

func ensureAvoidMasqRule() error {
	iptableHandler, err := iptables.New(ipt.ProtocolIPv4)
	if err != nil {
		return err
	}

	//ruleSpec := []string{"-m", "set", "--match-set", KosmosIPsetVoidMasq, "src",
	//	"-m", "set", "!", "--match-set", KosmosIPsetVoidMasq, "dst", "-j", "RETURN"}
	ruleSpec := []string{"-m", "set", "--match-set", KosmosIPsetVoidMasq, "dst", "-j", "RETURN"}

	if err = iptableHandler.InsertUnique("nat", "POSTROUTING", 2, ruleSpec); err != nil {
		return errors.Wrap(err, "unable to insert iptable rule in nat table to avoid masq")
	}
	return nil
}

func addIPset(ipsetcidr clusterlinkv1alpha1.IPset) error {
	if nmhelpers.GetIPType(ipsetcidr.CIDR) != nmhelpers.IPV4 {
		return fmt.Errorf("only support ipv4,can't avoid cidr %s masq", ipsetcidr.CIDR)
	}

	ipsetInterface := proxyipset.New(utilexec.New())

	_, err := netlink.IpsetList(ipsetcidr.Name)
	if err != nil {
		if !errors.Is(err, syscall.ENOENT) {
			return err
		}
		// kubeproxy ipset do not support hash:net type ipset,so use netlink
		err = netlink.IpsetCreate(ipsetcidr.Name, "hash:net", netlink.IpsetCreateOptions{})
		if err != nil {
			return err
		}
	}
	ipsetToAdd := ipset.IPSet{
		Name:       ipsetcidr.Name,
		HashFamily: ipset.ProtocolFamilyIPV4,
		SetType:    ipset.Type("hash:net"),
		MaxElem:    1048576,
		Comment:    "For kosmos",
	}
	//err := ipsetInterface.CreateSet(&ipsetToAdd, true)
	//if err != nil {
	//	return err
	//}

	// netlink don't support ipset protocol 7,so use kubeproxy way
	err = ipsetInterface.AddEntry(ipsetcidr.CIDR, &ipsetToAdd, true)
	if err != nil {
		return err
	}
	return nil
}

func deleteIPset(ipsetcidr clusterlinkv1alpha1.IPset) error {
	ipsetInterface := proxyipset.New(utilexec.New())

	err := ipsetInterface.DelEntry(ipsetcidr.CIDR, ipsetcidr.Name)
	if err != nil {
		return err
	}

	return nil
}

func loadIPsetAvoidMasq() ([]clusterlinkv1alpha1.IPset, error) {
	return ListIPset([]string{KosmosIPsetVoidMasq})
}

func ListIPset(ipsetListNames []string) ([]clusterlinkv1alpha1.IPset, error) {
	var errs error
	ret := []clusterlinkv1alpha1.IPset{}
	for _, ipsetName := range ipsetListNames {
		ipsetRet, err := netlink.IpsetList(ipsetName)
		if err != nil {
			if !errors.Is(err, syscall.ENOENT) {
				errs = errors.Wrap(err, fmt.Sprintf("error list ipset: %s", ipsetName))
			}
			continue
		}

		for _, entry := range ipsetRet.Entries {
			ret = append(ret, clusterlinkv1alpha1.IPset{
				Name: ipsetName,
				CIDR: fmt.Sprintf("%s/%d", entry.IP, entry.CIDR),
			})
		}
	}
	return ret, errs
}
