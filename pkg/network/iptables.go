package network

import (
	"fmt"
	"strings"

	ipt "github.com/coreos/go-iptables/iptables"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network/iptables"
)

type IptablesRecord struct {
	Table    string
	Chain    string
	Rule     []string
	Protocol ipt.Protocol
}

func (i *IptablesRecord) ToString() string {
	return fmt.Sprintf("iptables:%s_%s_%s_%v", i.Table, i.Chain, strings.Join(i.Rule, ""), i.Protocol)
}

// CreateGlobalNetIptablesChains 创建clusterLink自定义链
func CreateGlobalNetIptablesChains() error {

	klog.Infof("start to create globalnet chains")

	ipTypes := []ipt.Protocol{
		ipt.ProtocolIPv4,
		ipt.ProtocolIPv6,
	}

	for _, proto := range ipTypes {
		h, err := iptables.New(proto)
		if err != nil {
			return errors.Wrap(err, "failed to new iptables handler in CreateGlobalNetIptablesChains")
		}

		if err = h.CreateChainIfNotExists("nat", ClusterLinkPreRoutingChain); err != nil {
			return errors.Wrap(err, fmt.Sprintf("create %s err", ClusterLinkPreRoutingChain))
		}

		forwardToPreRoutingChain := []string{"-j", ClusterLinkPreRoutingChain}
		if err := h.PrependUnique("nat", "PREROUTING", forwardToPreRoutingChain); err != nil {
			return errors.Wrap(err, fmt.Sprintf("error appending iptables rule %q", strings.Join(forwardToPreRoutingChain, " ")))
		}

		if err = h.CreateChainIfNotExists("nat", ClusterLinkPostRoutingChain); err != nil {
			return errors.Wrap(err, fmt.Sprintf("create %s err", ClusterLinkPostRoutingChain))
		}

		forwardToPostRoutingChain := []string{"-j", ClusterLinkPostRoutingChain}
		if err := h.PrependUnique("nat", "POSTROUTING", forwardToPostRoutingChain); err != nil {
			return errors.Wrap(err, fmt.Sprintf("error appending iptables rule %q", strings.Join(forwardToPostRoutingChain, " ")))
		}
	}

	return nil
}

// ClearGlobalNetIptablesChains 清理clusterLink自定义链
func ClearGlobalNetIptablesChains(isIpv6 bool) error {

	ipType := ipt.ProtocolIPv4
	if isIpv6 {
		ipType = ipt.ProtocolIPv6
	}

	h, err := iptables.New(ipType)
	if err != nil {
		return errors.Wrap(err, "failed to new iptables handler in ClearGlobalNetIptablesChains")
	}

	if err := h.ClearChain("nat", ClusterLinkPreRoutingChain); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error while flushing rules in %s chain", ClusterLinkPreRoutingChain))
	}

	if err := h.ClearChain("nat", ClusterLinkPostRoutingChain); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error while flushing rules in %s chain", ClusterLinkPostRoutingChain))
	}

	return nil
}

func getRulesFromIptablesRecords(records []IptablesRecord) [][]string {
	rules := make([][]string, 0)
	for _, r := range records {
		rules = append(rules, r.Rule)
	}
	return rules
}

// updateIptablesWithInterface 将iptables分类并设置到底层
func updateIptablesWithInterface(i iptables.Interface, records []IptablesRecord) error {

	mapRecords := groupByTableChain(records)

	for key, items := range mapRecords {
		tableChain := strings.Split(key, "/")
		rules := getRulesFromIptablesRecords(items)

		if err := i.Append(tableChain[0], tableChain[1], rules[0]...); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to update iptables %s, rules: %v", tableChain, rules))
		}
	}

	return nil
}

// Delete(table, chain, ruleSpec...)
func deleteIptablesWithInterface(i iptables.Interface, records []IptablesRecord) error {

	mapRecords := groupByTableChain(records)

	for key, items := range mapRecords {
		tableChain := strings.Split(key, "/")
		rules := getRulesFromIptablesRecords(items)

		if err := i.Delete(tableChain[0], tableChain[1], rules[0]...); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to update iptables %s, rules: %v", tableChain, rules))
		}
	}

	return nil
}

func isIpt6(r clusterlinkv1alpha1.Iptables) bool {
	// TODO
	return strings.Contains(r.Rule, ":")
}

func translateChainName(key string, f bool) string {
	chainMap := map[string]string{
		"PREROUTING":  ClusterLinkPreRoutingChain,
		"POSTROUTING": ClusterLinkPostRoutingChain,
	}
	if f {
		return chainMap[key]
	} else {
		if chainMap["PREROUTING"] == key {
			return "PREROUTING"
		} else {
			return "POSTROUTING"
		}
	}
}

func groupByTableChain(records []IptablesRecord) map[string][]IptablesRecord {

	results := make(map[string][]IptablesRecord)
	for _, r := range records {
		tableChain := fmt.Sprintf("%s/%s", r.Table, translateChainName(r.Chain, true))
		results[tableChain] = append(results[tableChain], r)
	}
	return results
}

func loadIptables() ([]clusterlinkv1alpha1.Iptables, error) {

	ipts := []clusterlinkv1alpha1.Iptables{}

	ipTypes := []ipt.Protocol{
		ipt.ProtocolIPv4,
		ipt.ProtocolIPv6,
	}

	for _, proto := range ipTypes {
		h, err := iptables.New(proto)
		if err != nil {
			return nil, errors.Wrap(err, "failed to loadIptables")
		}
		chains := []string{ClusterLinkPreRoutingChain, ClusterLinkPostRoutingChain}
		for _, chainName := range chains {
			rules, err := h.List("nat", chainName)
			if err != nil {
				return nil, err
			}
			if len(rules) > 1 {
				for _, rule := range rules[1:] {
					ipts = append(ipts, clusterlinkv1alpha1.Iptables{
						Table: "nat",
						Chain: translateChainName(chainName, false),
						Rule:  strings.Join(strings.Split(rule, " ")[2:], " "),
					})
				}
			}
		}
	}

	return ipts, nil
}

// TODO: struce perfect
func addIptables(ipts clusterlinkv1alpha1.Iptables) error {
	klog.Infof("start to create globanet iptables: %v", ipts)

	if !isIpt6(ipts) {
		h, err := iptables.New(ipt.ProtocolIPv4)
		if err != nil {
			return fmt.Errorf("new iptablesHandler error, %v", err)
		}
		err = updateIptablesWithInterface(h, []IptablesRecord{
			{
				Table:    ipts.Table,
				Chain:    ipts.Chain,
				Rule:     strings.Split(ipts.Rule, " "),
				Protocol: ipt.ProtocolIPv4,
			},
		})
		if err != nil {
			return fmt.Errorf("update iptables error, %v", err)
		}
	} else {
		h, err := iptables.New(ipt.ProtocolIPv6)
		if err != nil {
			return fmt.Errorf("new iptablesHandler error, %v", err)
		}
		err = updateIptablesWithInterface(h, []IptablesRecord{
			{
				Table:    ipts.Table,
				Chain:    ipts.Chain,
				Rule:     strings.Split(ipts.Rule, " "),
				Protocol: ipt.ProtocolIPv6,
			},
		})
		if err != nil {
			return fmt.Errorf("update iptables error, %v", err)
		}
	}

	return nil
}

func deleteIptable(ipts clusterlinkv1alpha1.Iptables) error {
	klog.Infof("start to delete globanet iptables: %v", ipts)

	if !isIpt6(ipts) {
		h, err := iptables.New(ipt.ProtocolIPv4)
		if err != nil {
			return fmt.Errorf("new iptablesHandler error, %v", err)
		}
		err = deleteIptablesWithInterface(h, []IptablesRecord{
			{
				Table:    ipts.Table,
				Chain:    ipts.Chain,
				Rule:     strings.Split(ipts.Rule, " "),
				Protocol: ipt.ProtocolIPv4,
			},
		})
		if err != nil {
			return fmt.Errorf("update iptables error, %v", err)
		}
	} else {
		h, err := iptables.New(ipt.ProtocolIPv6)
		if err != nil {
			return fmt.Errorf("new iptablesHandler error, %v", err)
		}
		err = deleteIptablesWithInterface(h, []IptablesRecord{
			{
				Table:    ipts.Table,
				Chain:    ipts.Chain,
				Rule:     strings.Split(ipts.Rule, " "),
				Protocol: ipt.ProtocolIPv6,
			},
		})
		if err != nil {
			return fmt.Errorf("update iptables error, %v", err)
		}
	}

	return nil
}

func init() {
	if err := CreateGlobalNetIptablesChains(); err != nil {
		klog.Warning(err)
	}
}
