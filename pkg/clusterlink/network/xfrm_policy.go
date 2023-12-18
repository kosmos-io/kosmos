package network

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	log "k8s.io/klog/v2"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// For reference:
// https://github.com/flannel-io/flannel
/*
Use this func like linux command:

	ip xfrm policy add src $srcNet dst $dstNet dir $dir \
	                   tmpl src $srcIP dst $dstIP proto esp reqid $reqID mode tunnel
*/
func AddXFRMPolicy(srcNet, dstNet *net.IPNet, srcIP, dstIP net.IP, dir netlink.Dir, reqID int) error {
	policy := &netlink.XfrmPolicy{
		Src: srcNet,
		Dst: dstNet,
		Dir: dir,
	}

	tmpl := netlink.XfrmPolicyTmpl{
		Src:   srcIP,
		Dst:   dstIP,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Reqid: reqID,
	}

	policy.Tmpls = append(policy.Tmpls, tmpl)

	if existingPolicy, err := netlink.XfrmPolicyGet(policy); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			log.Infof("Adding ipsec policy: %+v", tmpl)
			if err := netlink.XfrmPolicyAdd(policy); err != nil {
				return fmt.Errorf("error adding policy: %+v err: %v", policy, err)
			}
		} else {
			return fmt.Errorf("error getting policy: %+v err: %v", policy, err)
		}
	} else {
		log.Infof("Updating ipsec policy %+v with %+v", existingPolicy, policy)
		if err := netlink.XfrmPolicyUpdate(policy); err != nil {
			return fmt.Errorf("error updating policy: %+v err: %v", policy, err)
		}
	}
	return nil
}

/*
Use this func like linux command:

	ip xfrm policy del src $srcNet dst $dstNet dir $dir \
	                   tmpl src $srcIP dst $dstIP proto esp reqid $reqID mode tunnel
*/
func DeleteXFRMPolicy(srcNet, dstNet *net.IPNet, srcIP, dstIP net.IP, dir netlink.Dir, reqID int) error {
	policy := netlink.XfrmPolicy{
		Src: srcNet,
		Dst: dstNet,
		Dir: dir,
	}

	tmpl := netlink.XfrmPolicyTmpl{
		Src:   srcIP,
		Dst:   dstIP,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Reqid: reqID,
	}

	log.Infof("Deleting ipsec policy: %+v", tmpl)

	policy.Tmpls = append(policy.Tmpls, tmpl)

	if err := netlink.XfrmPolicyDel(&policy); err != nil {
		return fmt.Errorf("error deleting policy: %+v err: %v", policy, err)
	}

	return nil
}

/*
Use this func like linux command:

	ip xfrm state add src $srcIP dst $dstIP proto esp spi $spi reqid $reqID mode tunnel aead 'rfc4106(gcm(aes))' $psk 128
*/
func AddXFRMState(srcIP, dstIP net.IP, reqID int, spi int, psk string) error {
	k, _ := hex.DecodeString(psk)
	state := netlink.XfrmState{
		Src:   srcIP,
		Dst:   dstIP,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Spi:   spi,
		Reqid: reqID,
		Aead: &netlink.XfrmStateAlgo{
			Name:   "rfc4106(gcm(aes))",
			Key:    k,
			ICVLen: 128,
		},
	}

	if existingState, err := netlink.XfrmStateGet(&state); err != nil {
		if errors.Is(err, syscall.ESRCH) || errors.Is(err, syscall.ENOENT) {
			log.Infof("Adding xfrm state: %+v", state)
			if err := netlink.XfrmStateAdd(&state); err != nil {
				return fmt.Errorf("error adding state: %+v err: %v", state, err)
			}
		} else {
			return fmt.Errorf("error getting state: %+v err: %v", state, err)
		}
	} else {
		log.Infof("Updating xfrm state %+v with %+v", existingState, state)
		if err := netlink.XfrmStateUpdate(&state); err != nil {
			return fmt.Errorf("error updating state: %+v err: %v", state, err)
		}
	}
	return nil
}

/*
Use this func like linux command:

	ip xfrm state del src $srcIP dst $dstIP proto esp spi $spi reqid $reqID mode tunnel aead 'rfc4106(gcm(aes))' $psk 128
*/
func DeleteXFRMState(srcIP, dstIP net.IP, reqID int, spi int, psk string) error {
	k, _ := hex.DecodeString(psk)
	state := netlink.XfrmState{
		Src:   srcIP,
		Dst:   dstIP,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Spi:   spi,
		Reqid: reqID,
		Aead: &netlink.XfrmStateAlgo{
			Name:   "rfc4106(gcm(aes))",
			Key:    k,
			ICVLen: 128,
		},
	}
	log.Infof("Deleting ipsec state: %+v", state)
	err := netlink.XfrmStateDel(&state)
	if err != nil {
		return fmt.Errorf("error delete xfrm state: %+v err: %v", state, err)
	}
	return nil
}

func ListXfrmPolicy() ([]clusterlinkv1alpha1.XfrmPolicy, error) {
	xfrmpolicies, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("error list xfrm policy: %v", err)
	}
	var ret []clusterlinkv1alpha1.XfrmPolicy
	for _, policy := range xfrmpolicies {
		ret = append(ret, clusterlinkv1alpha1.XfrmPolicy{
			LeftIP:   policy.Tmpls[0].Src.String(),
			LeftNet:  policy.Src.String(),
			RightIP:  policy.Tmpls[0].Dst.String(),
			RightNet: policy.Dst.String(),
			ReqID:    policy.Tmpls[0].Reqid,
			Dir:      int(policy.Dir),
		})
	}
	return ret, nil
}

func ListXfrmState() ([]clusterlinkv1alpha1.XfrmState, error) {
	xfrmstates, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("error list xfrm state: %v", err)
	}
	var ret []clusterlinkv1alpha1.XfrmState
	for _, state := range xfrmstates {
		k := hex.EncodeToString(state.Aead.Key)
		ret = append(ret, clusterlinkv1alpha1.XfrmState{
			LeftIP:  state.Src.String(),
			RightIP: state.Dst.String(),
			ReqID:   state.Reqid,
			PSK:     k,
			SPI:     uint32(state.Spi),
		})
	}
	return ret, nil
}

func loadXfrmPolicy() ([]clusterlinkv1alpha1.XfrmPolicy, error) {
	return ListXfrmPolicy()
}

func loadXfrmState() ([]clusterlinkv1alpha1.XfrmState, error) {
	return ListXfrmState()
}
