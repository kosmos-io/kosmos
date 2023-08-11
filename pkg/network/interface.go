package network

import (
	"errors"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

var (
	// ErrNotImplemented is returned when a requested feature is not implemented.
	ErrNotImplemented = errors.New("not implemented")
)

type NetWork interface {
	LoadSysConfig() (*clusterlinkv1alpha1.NodeConfigSpec, error)

	DeleteArps([]clusterlinkv1alpha1.Arp) error
	DeleteFdbs([]clusterlinkv1alpha1.Fdb) error
	DeleteIptables([]clusterlinkv1alpha1.Iptables) error
	DeleteRoutes([]clusterlinkv1alpha1.Route) error
	DeleteDevices([]clusterlinkv1alpha1.Device) error

	UpdateArps([]clusterlinkv1alpha1.Arp) error
	UpdateFdbs([]clusterlinkv1alpha1.Fdb) error
	UpdateIptables([]clusterlinkv1alpha1.Iptables) error
	UpdateRoutes([]clusterlinkv1alpha1.Route) error
	UpdateDevices([]clusterlinkv1alpha1.Device) error

	AddArps([]clusterlinkv1alpha1.Arp) error
	AddFdbs([]clusterlinkv1alpha1.Fdb) error
	AddIptables([]clusterlinkv1alpha1.Iptables) error
	AddRoutes([]clusterlinkv1alpha1.Route) error
	AddDevices([]clusterlinkv1alpha1.Device) error
}

func NewNetWork() NetWork {
	return &DefaultNetWork{}
}
