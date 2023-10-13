package network

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	ipt "github.com/coreos/go-iptables/iptables"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/network/iptables"
)

func UpdateDefaultIp6tablesBehavior(ifaceName string) error {
	iptableHandler, err := iptables.New(ipt.ProtocolIPv6)
	if err != nil {
		return err //nolint:wrapcheck  // Let the caller wrap it
	}

	ruleSpec := []string{"-o", ifaceName, "-j", "ACCEPT"}

	if err = iptableHandler.PrependUnique("filter", "FORWARD", ruleSpec); err != nil {
		return errors.Wrap(err, "unable to insert iptable rule in filter table to allow vxlan traffic")
	}

	return nil
}

func UpdateDefaultIp4tablesBehavior(ifaceName string) error {
	iptableHandler, err := iptables.New(ipt.ProtocolIPv4)
	if err != nil {
		return err //nolint:wrapcheck  // Let the caller wrap it
	}

	ruleSpec := []string{"-o", ifaceName, "-j", "ACCEPT"}

	if err = iptableHandler.PrependUnique("filter", "FORWARD", ruleSpec); err != nil {
		return errors.Wrap(err, "unable to insert iptable rule in filter table to allow vxlan traffic")
	}

	return nil
}

// submariner\pkg\netlink\netlink.go
func setSysctl(path string, contents []byte) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Ignore leading and terminating newlines
	existing = bytes.Trim(existing, "\n")

	if bytes.Equal(existing, contents) {
		return nil
	}
	// Permissions are already 644, the files are never created
	// #nosec G306
	return os.WriteFile(path, contents, 0o644)
}

func EnableLooseModeByIFaceNmae(ifaceName string) error {
	// Enable loose mode (rp_filter=2) reverse path filtering on the vxlan interface.
	err := setSysctl("/proc/sys/net/ipv4/conf/"+ifaceName+"/rp_filter", []byte("2"))
	return errors.Wrapf(err, "unable to update rp_filter proc entry for interface %q", ifaceName)
}

func EnableDisableIPV6ByIFaceNmae(ifaceName string) error {
	// Enable ipv6 (disable_ipv6=0)
	err := setSysctl("/proc/sys/net/ipv6/conf/"+ifaceName+"/disable_ipv6", []byte("0"))
	return errors.Wrapf(err, "unable to update disable_ipv6 proc entry for interface %q", ifaceName)
}

// set flannel interface (rp_filter=2)
func EnableLooseModeForFlannel() error {
	links, err := netlink.LinkList()
	if err != nil {
		return err
	}

	var errs error
	for _, link := range links {
		linkName := link.Attrs().Name
		if strings.HasPrefix(linkName, FLANNEL_DEV_NAME_PREFIX) {
			if err := setSysctl("/proc/sys/net/ipv4/conf/"+linkName+"/rp_filter", []byte("2")); err != nil {
				errs = errors.Wrapf(errors.Wrapf(err, "unable to update flannel rp_filter proc entry for interface %q", linkName), fmt.Sprint(errs))
			}
		}
	}

	return errs
}
