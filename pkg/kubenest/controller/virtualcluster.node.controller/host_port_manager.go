package vcnodecontroller

import (
	"context"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

/**
apiVersion: v1
kind: ConfigMap
metadata:
  name: kosmos-hostports
  namespace: kosmos-system
data:
  config.yaml: |
    PortsPool:
      - 5443
      - 6443
      - 7443
    ClusterPorts:
      - Port: 5443
        Cluster: "cluster1"
      - Port: 6443
        Cluster: "cluster2"
*/

type HostPortManager struct {
	HostPortPool *HostPortPool
	kubeClient   kubernetes.Interface
	lock         sync.Mutex
}

type HostPortPool struct {
	PortsPool    []int32       `yaml:"portsPool"`
	ClusterPorts []ClusterPort `yaml:"clusterPorts"`
}

type ClusterPort struct {
	Port    int32  `yaml:"port"`
	Cluster string `yaml:"cluster"`
}

func NewHostPortManager(client kubernetes.Interface) (*HostPortManager, error) {
	//todo magic Value
	hostPorts, err := client.CoreV1().ConfigMaps("kosmos-system").Get(context.TODO(), "kosmos-hostports", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	//todo magic Value
	yamlData, exist := hostPorts.Data["config.yaml"]
	if !exist {
		return nil, fmt.Errorf("hostports not found in configmap")
	}

	var hostPool HostPortPool
	if err := yaml.Unmarshal([]byte(yamlData), &hostPool); err != nil {
		return nil, err
	}
	manager := &HostPortManager{
		HostPortPool: &hostPool,
		kubeClient:   client,
	}
	return manager, nil
}

func (m *HostPortManager) AllocateHostIP(clusterName string) (int32, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, port := range m.HostPortPool.PortsPool {
		if !m.isPortAllocated(port) {
			m.HostPortPool.ClusterPorts = append(m.HostPortPool.ClusterPorts, ClusterPort{Port: port, Cluster: clusterName})
			m.HostPortPool.ClusterPorts = append(m.HostPortPool.ClusterPorts, ClusterPort{Port: port, Cluster: clusterName})
			return port, nil
		}
	}
	// todo 更新 cm
	return 0, fmt.Errorf("no available ports to allocate")
}

func (m *HostPortManager) ReleaseHostIP(clusterName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i, cp := range m.HostPortPool.ClusterPorts {
		if cp.Cluster == clusterName {
			// Remove the entry from the slice
			m.HostPortPool.ClusterPorts = append(m.HostPortPool.ClusterPorts[:i], m.HostPortPool.ClusterPorts[i+1:]...)
			return nil
		}
	}
	// todo 更新 cm
	return fmt.Errorf("no port found for cluster %s", clusterName)
}

func (m *HostPortManager) isPortAllocated(port int32) bool {
	for _, cp := range m.HostPortPool.ClusterPorts {
		if cp.Port == port {
			return true
		}
	}
	return false
}
