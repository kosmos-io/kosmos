package controller

import (
	"fmt"
	"testing"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"k8s.io/klog/v2"
)

func isPortAllocated(port int32, ports map[string]int32) bool {
	// vcList := &v1alpha1.VirtualClusterList{}
	// if err != nil {
	// 	klog.Errorf("list virtual cluster error: %v", err)
	// 	return false
	// }

	// 判断一个map是否包含某个端口

	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false

}

func AllocateHostPortTemplate(virtualCluster *v1alpha1.VirtualCluster, usedPorts map[string]int32, hostPoolPorts ...int32) (map[string]int32, error) {

	if len(virtualCluster.Status.PortMap) > 0 || virtualCluster.Status.Port != 0 {
		return nil, nil
	}
	// 获取主机端口池

	isPortInPool := func(port1 int32) bool {
		for _, p := range hostPoolPorts {
			if int32(p) == port1 {
				return true
			}
		}
		return false
	}

	// 准备端口分配列表

	//判断ExternalPort是否在主机端口池里面
	if virtualCluster.Spec.ExternalPort != 0 && !isPortInPool(virtualCluster.Spec.ExternalPort) {
		return nil, nil
	}

	ports := func() []int32 {
		ports := make([]int32, 0)
		if virtualCluster.Spec.ExternalPort != 0 && !isPortAllocated(virtualCluster.Spec.ExternalPort, usedPorts) {
			ports = append(ports, virtualCluster.Spec.ExternalPort)
		} else if isPortAllocated(virtualCluster.Spec.ExternalPort, usedPorts) {
			return nil
		}
		for _, p := range hostPoolPorts {
			if !isPortAllocated(p, usedPorts) {
				ports = append(ports, p)
			}
		}
		return ports
	}()

	if ports == nil {
		return nil, fmt.Errorf("port is already being used")
	}
	//可分配端口不够
	if len(ports) < constants.VirtualClusterPortNum {
		return nil, fmt.Errorf("no available ports to allocate")
	}
	// 分配端口并s更新 PortMap
	virtualCluster.Status.PortMap = make(map[string]int32)
	virtualCluster.Status.PortMap[constants.ApiServerPortKey] = ports[0]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyAgentPortKey] = ports[1]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyServerPortKey] = ports[2]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyHealthPortKey] = ports[3]
	virtualCluster.Status.PortMap[constants.ApiServerNetworkProxyAdminPortKey] = ports[4]

	klog.V(4).InfoS("Success allocate virtual cluster ports", "allocate ports", ports, "vc ports", ports[:2])

	return virtualCluster.Status.PortMap, nil
}
func CompareMap(a map[string]int32, b map[string]int32) bool {
	// 1. 检查长度是否相同
	if len(a) != len(b) {
		return false
	}

	// 2. 遍历第一个 map，并检查每个键值对在第二个 map 中是否存在并且相等
	for key, valueA := range a {
		valueB, ok := b[key]
		if !ok || valueA != valueB {
			return false
		}
	}

	// 3. 如果通过了所有检查，返回 true，表示两个 map 相等
	return true
}

func TestVirtualClusterInitController_AllocateHostPort(t *testing.T) {
	type args struct {
		externalPort int32
		ports        []int32
		usedPorts    map[string]int32
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]int32
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				externalPort: 33010,
				ports:        []int32{33001, 33002, 33003, 33004, 33005, 33006, 33007, 33008, 33009, 33010},
				usedPorts:    map[string]int32{"vc1": 33001, "vc2": 33002, "vc3": 33003},
			},
			want: map[string]int32{"apiserver-port": 33010, "apiserver-network-proxy-agent-port": 33004, "apiserver-network-proxy-server-port": 33005, "apiserver-network-proxy-health-port": 33006, "apiserver-network-proxy-admin-port": 33007},
		},
		{
			name: "test2",
			args: args{
				// externalPort: 33010,
				ports:     []int32{33001, 33002, 33003, 33004, 33005, 33006, 33007, 33008, 33009, 33010},
				usedPorts: map[string]int32{"vc1": 33006, "vc2": 33002, "vc3": 33003},
			},
			want: map[string]int32{"apiserver-port": 33001, "apiserver-network-proxy-agent-port": 33004, "apiserver-network-proxy-server-port": 33005, "apiserver-network-proxy-health-port": 33007, "apiserver-network-proxy-admin-port": 33008},
		},
		{
			name: "test3 - Not enough ports available",
			args: args{
				externalPort: 33001,
				ports:        []int32{33001, 33002, 33003, 33004},
				usedPorts:    map[string]int32{"vc1": 33001},
			},
			want: nil,
		},
		{
			name: "test4 - Ports already allocated",
			args: args{
				externalPort: 33000,
				ports:        []int32{33001, 33002, 33003, 33004, 33005, 33006, 33007},
				usedPorts:    map[string]int32{},
			},
			want: nil,
		},
		{
			name: "test5 ",
			args: args{
				externalPort: 33001,
				ports:        []int32{33001, 33002, 33003, 33004, 33005, 33006, 33007},
				usedPorts:    map[string]int32{"vc1": 33001},
			},
			want: nil,
		},
		{
			name: "test6",
			args: args{
				ports:     []int32{33001, 33002, 33003, 33004, 33005, 33006, 33007},
				usedPorts: map[string]int32{},
			},
			want: map[string]int32{"apiserver-port": 33001, "apiserver-network-proxy-agent-port": 33002, "apiserver-network-proxy-server-port": 33003, "apiserver-network-proxy-health-port": 33004, "apiserver-network-proxy-admin-port": 33005},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			virtualClusterTest := &v1alpha1.VirtualCluster{}
			virtualClusterTest.Spec.ExternalPort = tt.args.externalPort
			got, _ := AllocateHostPortTemplate(virtualClusterTest, tt.args.usedPorts, tt.args.ports...)
			// if (err != nil) != tt.wantErr {
			// 	t.Errorf("VirtualClusterInitController.AllocateHostPort() error = %v, wantErr %v", err, tt.wantErr)
			// 	return
			// }
			if !CompareMap(got, tt.want) {
				t.Errorf("VirtualClusterInitController.AllocateHostPort() = %v, want %v", got, tt.want)
			} else {
				fmt.Println(tt.name + "success")
			}
		})
	}
}
