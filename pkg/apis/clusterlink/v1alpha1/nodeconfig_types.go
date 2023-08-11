package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeConfigSpec `json:"spec"`

	// +optional
	Status NodeConfigStatus `json:"status,omitempty"`
}

type NodeConfigSpec struct {
	Devices  []Device   `json:"devices,omitempty"`
	Routes   []Route    `json:"routes,omitempty"`
	Iptables []Iptables `json:"iptables,omitempty"`
	Fdbs     []Fdb      `json:"fdbs,omitempty"`
	Arps     []Arp      `json:"arps,omitempty"`
}

type NodeConfigStatus struct {
	LastChangeTime metav1.Time `json:"lastChangeTime,omitempty"`
	LastSyncTime   metav1.Time `json:"lastSyncTime,omitempty"`
}

type Device struct {
	Type    DeviceType `json:"type"`
	Name    string     `json:"name"`
	Addr    string     `json:"addr"`
	Mac     string     `json:"mac"`
	BindDev string     `json:"bindDev"`
	ID      int32      `json:"id"`
	Port    int32      `json:"port"`
}

func (d *Device) Compare(v Device) bool {
	return d.Type == v.Type &&
		d.Name == v.Name &&
		d.Addr == v.Addr &&
		d.Mac == v.Mac &&
		d.BindDev == v.BindDev &&
		d.ID == v.ID &&
		d.Port == v.Port
}

type Route struct {
	CIDR string `json:"cidr"`
	Gw   string `json:"gw"`
	Dev  string `json:"dev"`
}

func (r *Route) Compare(v Route) bool {
	return r.CIDR == v.CIDR &&
		r.Gw == v.Gw &&
		r.Dev == v.Dev
}

type Iptables struct {
	Table string `json:"table"`
	Chain string `json:"chain"`
	Rule  string `json:"rule"`
}

func (i *Iptables) Compare(v Iptables) bool {
	return i.Table == v.Table &&
		i.Chain == v.Chain &&
		i.Rule == v.Rule
}

type Fdb struct {
	IP  string `json:"ip"`
	Mac string `json:"mac"`
	Dev string `json:"dev"`
}

func (f *Fdb) Compare(v Fdb) bool {
	return f.IP == v.IP &&
		f.Mac == v.Mac &&
		f.Dev == v.Dev
}

type Arp struct {
	IP  string `json:"ip"`
	Mac string `json:"mac"`
	Dev string `json:"dev"`
}

func (a *Arp) Compare(v Arp) bool {
	return a.IP == v.IP &&
		a.Mac == v.Mac &&
		a.Dev == v.Dev
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NodeConfig `json:"items"`
}
