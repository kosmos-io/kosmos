package ironicparametercontroller

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
)

type IronicParameterController struct {
	client.Client
	Config        *rest.Config
	RootClientSet kubernetes.Interface
	EventRecorder record.EventRecorder
}

type IronicNpuTor struct {
	NodeName        string
	IronicDeviceId  string
	IronicPortId    string
	TorSwitchIpList []string
	FixIp           string
	NpuInfo         []NpuInfo
	NpuDeviceIpList []string
}

type NpuInfo struct {
	Id         string `json:"id"`
	SwitchIp   string `json:"switch_ip"`
	MacAddress string `json:"mac_address"`
	PortName   string `json:"port_name"`
	IPAddress  string `json:"ip_address"`
	Gateway    string `json:"gateway"`
	Mask       int    `json:"mask"`
}

func (i *IronicParameterController) SetupWithManager(mgr manager.Manager) error {
	if i.Client == nil {
		i.Client = mgr.GetClient()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(constants.GlobalNodeControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&v1.ConfigMap{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
		})).
		Complete(i)
}

func (i *IronicParameterController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("============ ironic-parameter-controller start to reconcile %s ============", request.NamespacedName)
	defer klog.V(4).Infof("============ ironic-parameter-controller finish to reconcile %s ============", request.NamespacedName)
	// TODO: add your logic here
	return reconcile.Result{}, nil
}

func (i *IronicParameterController) GetGlobalnodesIPByName(ctx context.Context, nodename string) (string, error) {
	var globalNode v1alpha1.GlobalNode
	if err := i.Get(ctx, types.NamespacedName{
		Name:      nodename,
		Namespace: "default",
	}, &globalNode); err != nil {
		return "", err
	}
	return globalNode.Spec.NodeIP, nil
}

func parserMac(input string) (string, error) {
	parts := strings.Split(input, ": ")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid mac address: %s", input)
	}
	mac := parts[1]
	if len(mac) != 17 {
		return "", fmt.Errorf("invalid mac address: %s", input)
	}
	return mac, nil
}

func (i *IronicParameterController) GetNodeMacList(ctx context.Context, exectHelper *exector.ExectorHelper) ([]string, error) {
	var macs []string
	for i := 0; i < 8; i++ {
		checkCmd := &exector.CMDExector{
			Cmd: fmt.Sprintf("hccn_tool -i %d -mac -g", i),
		}
		ret := exectHelper.DoExector(ctx.Done(), checkCmd)
		if ret.Status != exector.SUCCESS {
			return nil, fmt.Errorf("get node mac list failed: %s", ret.String())
		}
		mac, err := parserMac(ret.LastLog)
		if err != nil {
			return nil, fmt.Errorf("get node mac list failed: %s", err.Error())
		}
		macs = append(macs, mac)
	}

	return macs, nil
}

func (i *IronicParameterController) findNpuInfo(mac string, npuInfos []NpuInfo) (*NpuInfo, error) {
	for _, npuInfo := range npuInfos {
		if npuInfo.MacAddress == mac {
			return &npuInfo, nil
		}
	}
	return nil, fmt.Errorf("cannot find npuinfo for mac: %s", mac)
}

func getMacByBytes(mask int) (string, error) {
	if mask < 0 || mask > 32 {
		return "", fmt.Errorf("invalid subnet mask length: %d", mask)
	}
	fullMask := net.CIDRMask(mask, 32)

	var maskParts []string
	for _, b := range fullMask {
		maskParts = append(maskParts, strconv.Itoa(int(b)))
	}

	return strings.Join(maskParts, "."), nil
}

func (i *IronicParameterController) SetNetwork(ctx context.Context, exectHelper *exector.ExectorHelper, macs []string, npuInfos []NpuInfo) error {
	for index, mac := range macs {
		npuInfo, err := i.findNpuInfo(mac, npuInfos)
		if err != nil {
			return fmt.Errorf("set network failed, mac: %s, err: %s", mac, err.Error())
		}
		mask, err := getMacByBytes(npuInfo.Mask)
		if err != nil {
			return fmt.Errorf("set network failed, mac: %s, err: %s", mac, err.Error())
		}
		// TODO: add retry
		tasks := []*exector.CMDExector{
			{
				Cmd: fmt.Sprintf("hccn_tool -i %d -ip -s address %s netmask %s", index, npuInfo.IPAddress, mask),
			},
			{
				Cmd: fmt.Sprintf("hccn_tool -i %d -gateway -s gateway %s", index, npuInfo.Gateway),
			},
			{
				Cmd: fmt.Sprintf("hccn_tool -i %d -netdetect -s address %s", index, npuInfo.Gateway),
			},
		}

		for _, task := range tasks {
			ret := exectHelper.DoExector(ctx.Done(), task)
			if ret.Status != exector.SUCCESS {
				return fmt.Errorf("set network failed, mac: %s, err: %s", mac, ret.String())
			}
		}
	}
	return nil
}

func (i *IronicParameterController) DoTask(ctx context.Context, ironicNpuTour *IronicNpuTor) error {
	nodeIP, err := i.GetGlobalnodesIPByName(ctx, ironicNpuTour.NodeName)
	if err != nil {
		return nil
	}

	exectHelper := exector.NewExectorHelperForHccn(nodeIP, "")

	macs, err := i.GetNodeMacList(ctx, exectHelper)
	if err != nil {
		return fmt.Errorf("do task failed, nodename: %s, err: %s", ironicNpuTour.NodeName, err.Error())
	}

	err = i.SetNetwork(ctx, exectHelper, macs, ironicNpuTour.NpuInfo)

	if err != nil {
		return fmt.Errorf("do task failed, nodename: %s, err: %s", ironicNpuTour.NodeName, err.Error())
	}

	return nil
}
