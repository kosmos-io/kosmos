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
	Devices          []Device     `json:"devices,omitempty"`
	Routes           []Route      `json:"routes,omitempty"`
	Iptables         []Iptables   `json:"iptables,omitempty"`
	Fdbs             []Fdb        `json:"fdbs,omitempty"`
	Arps             []Arp        `json:"arps,omitempty"`
	XfrmPolicies     []XfrmPolicy `json:"xfrmpolicies,omitempty"`
	XfrmStates       []XfrmState  `json:"xfrmstates,omitempty"`
	IPsetsAvoidMasqs []IPset      `json:"ipsetsavoidmasq,omitempty"`
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

/*
Use this struct like linux command:

	ip xfrm policy add src $LeftNet dst $RightNet dir $Dir \
	                   tmpl src $LeftIP dst $RightIP proto esp reqid $ReqID mode tunnel
	ip xfrm policy del src $LeftNet dst $RightNet dir $Dir \
	                   tmpl src $LeftIP dst $RightIP proto esp reqid $ReqID mode tunnel
*/
type XfrmPolicy struct {
	LeftIP   string `json:"leftip"`
	LeftNet  string `json:"leftnet"`
	RightIP  string `json:"rightip"`
	RightNet string `json:"rightnet"`
	ReqID    int    `json:"reqid"`
	Dir      int    `json:"dir"`
}

func (a *XfrmPolicy) Compare(v XfrmPolicy) bool {
	return a.LeftIP == v.LeftIP &&
		a.LeftNet == v.LeftNet &&
		a.RightNet == v.RightNet &&
		a.RightIP == v.RightIP &&
		a.ReqID == v.ReqID &&
		a.Dir == v.Dir
}

/*
Use this struct like linux command:

	ip xfrm state add src $LeftIP dst $RightIP proto esp spi $ID reqid $ID mode tunnel aead 'rfc4106(gcm(aes))' $PSK 128
	ip xfrm state del src $LeftIP dst $RightIP proto esp spi $ID reqid $ID mode tunnel aead 'rfc4106(gcm(aes))' $PSK 128
*/
type XfrmState struct {
	LeftIP  string `json:"leftip"`
	RightIP string `json:"rightip"`
	ReqID   int    `json:"reqid"`
	SPI     uint32 `json:"spi"`
	PSK     string `json:"PSK"`
}

func (a *XfrmState) Compare(v XfrmState) bool {
	return a.LeftIP == v.LeftIP &&
		a.RightIP == v.RightIP &&
		a.ReqID == v.ReqID &&
		a.PSK == v.PSK &&
		a.SPI == v.SPI
}

type IPset struct {
	CIDR string `json:"cidr"`
	Name string `json:"name"`
}

func (a *IPset) Compare(v IPset) bool {
	return a.CIDR == v.CIDR &&
		a.Name == v.Name
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NodeConfig `json:"items"`
}
