package helpers

import (
	"encoding/json"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// RouteSorter sorts routes.
type RouteSorter []v1alpha1.Route

func (s RouteSorter) Len() int      { return len(s) }
func (s RouteSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s RouteSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// IptablesSorter sorts iptables.
type IptablesSorter []v1alpha1.Iptables

func (s IptablesSorter) Len() int      { return len(s) }
func (s IptablesSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s IptablesSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// DevicesSorter sorts devices.
type DevicesSorter []v1alpha1.Device

func (s DevicesSorter) Len() int      { return len(s) }
func (s DevicesSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s DevicesSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// ArpSorter sorts iptables.
type ArpSorter []v1alpha1.Arp

func (s ArpSorter) Len() int      { return len(s) }
func (s ArpSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ArpSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// FdbSorter sorts iptables.
type FdbSorter []v1alpha1.Fdb

func (s FdbSorter) Len() int      { return len(s) }
func (s FdbSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s FdbSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// XfrmPolicySorter sorts xfrm policy.
type XfrmPolicySorter []v1alpha1.XfrmPolicy

func (s XfrmPolicySorter) Len() int      { return len(s) }
func (s XfrmPolicySorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s XfrmPolicySorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// XfrmStateorter sorts xfrm policy.
type XfrmStateSorter []v1alpha1.XfrmState

func (s XfrmStateSorter) Len() int      { return len(s) }
func (s XfrmStateSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s XfrmStateSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}

// IPsetsorter sorts xfrm policy.
type IPSetSorter []v1alpha1.IPset

func (s IPSetSorter) Len() int      { return len(s) }
func (s IPSetSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s IPSetSorter) Less(i, j int) bool {
	strI, err := json.Marshal(s[i])
	if err != nil {
		return i < j
	}
	strJ, err := json.Marshal(s[j])
	if err != nil {
		return i < j
	}
	return string(strI) > string(strJ)
}
