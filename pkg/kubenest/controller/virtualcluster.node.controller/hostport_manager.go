package vcnodecontroller

import (
	"context"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
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
	hostPorts, err := client.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.TODO(), constants.HostPortsCMName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	yamlData, exist := hostPorts.Data[constants.HostPortsCMDataName]
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

func (m *HostPortManager) AllocateHostPort(clusterName string) (int32, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	//使用临时变量存储原来的cm
	oldHostPool := m.HostPortPool

	for _, port := range m.HostPortPool.PortsPool {
		if !m.isPortAllocated(port) {
			m.HostPortPool.ClusterPorts = append(m.HostPortPool.ClusterPorts, ClusterPort{Port: port, Cluster: clusterName})
			err := updateConfigMapAndRollback(m, oldHostPool)
			if err != nil {
				return 0, err
			}
			return port, err
		}
	}
	return 0, fmt.Errorf("no available ports to allocate")
}

func (m *HostPortManager) ReleaseHostPort(clusterName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	oldHostPool := m.HostPortPool

	for i, cp := range m.HostPortPool.ClusterPorts {
		if cp.Cluster == clusterName {
			// Remove the entry from the slice
			m.HostPortPool.ClusterPorts = append(m.HostPortPool.ClusterPorts[:i], m.HostPortPool.ClusterPorts[i+1:]...)
			err := updateConfigMapAndRollback(m, oldHostPool)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("no port found for cluster %s", clusterName)
}

func updateConfigMapAndRollback(m *HostPortManager, oldHostPool *HostPortPool) error {
	data, err := yaml.Marshal(m.HostPortPool)
	if err != nil {
		m.HostPortPool = oldHostPool
		return err
	}

	configMap, err := m.kubeClient.CoreV1().ConfigMaps(constants.KosmosNs).Get(context.TODO(), constants.HostPortsCMName, metav1.GetOptions{})
	if err != nil {
		m.HostPortPool = oldHostPool
		return err
	}

	configMap.Data[constants.HostPortsCMDataName] = string(data)

	_, updateErr := m.kubeClient.CoreV1().ConfigMaps(constants.KosmosNs).Update(context.TODO(), configMap, metav1.UpdateOptions{})

	if updateErr != nil {
		// 回滚 HostPortPool
		m.HostPortPool = oldHostPool
		return updateErr
	}

	return nil
}

func (m *HostPortManager) isPortAllocated(port int32) bool {
	for _, cp := range m.HostPortPool.ClusterPorts {
		if cp.Port == port {
			return true
		}
	}
	return false
}
