package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/projectcalico/calico/libcalico-go/lib/apiconfig"
	"github.com/projectcalico/calico/libcalico-go/lib/clientv3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

const (
	DataStoreType = "datastoreType"
	EtcdV3        = "etcdv3"

	ServiceClusterIpRange = "--service-cluster-ip-range"
)

type CalicoConfig struct {
	DatastoreType string `json:"datastoreType"`

	EtcdEndpoints string `json:"etcdEndpoints"`
	EtcdKey       string `json:"etcdKey"`
	EtcdCert      string `json:"etcdCert"`
	EtcdCACert    string `json:"etcdCACert"`
}

func CheckIsEtcd(cluster *clusterlinkv1alpha1.Cluster) bool {
	storeType := cluster.Annotations[DataStoreType]
	switch storeType {
	case EtcdV3:
		return true
	default:
		return false
	}
}

func GetCalicoClient(cluster *clusterlinkv1alpha1.Cluster) (clientv3.Interface, error) {
	var calicoConfig CalicoConfig
	config, err := ctrl.GetConfig()
	if err != nil {
		klog.Errorf("failed to get k8s config: %v", err)
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("failed to build k8s kubeClient: %v", err)
		return nil, err
	}

	clusterConfigMap, err := clientSet.CoreV1().ConfigMaps("clusterlink-system").Get(context.Background(), cluster.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get cluster configmap, cluster name is %s.", cluster.Name)
		return nil, err
	}

	calicoAPIConfig := apiconfig.NewCalicoAPIConfig()
	calicoData := clusterConfigMap.Data
	calicoJsonStr, err := json.Marshal(calicoData)
	if err != nil {
		klog.Errorf("failed to marshal cluster configmap %s to json string.", cluster.Name)
		return nil, err
	}
	err = json.Unmarshal(calicoJsonStr, &calicoConfig)
	if err != nil {
		klog.Errorf("failed to unmarshal json string to calico config, cluster configmap is %s.", cluster.Name)
		return nil, err
	}

	// Decoding etcd config.
	decodeEtcdKey, err := base64.StdEncoding.DecodeString(calicoConfig.EtcdKey)
	if err != nil {
		klog.Errorf("decoding exception, etcd key is invalid, cluster name is %s.", cluster.Name)
		return nil, err
	}
	decodeEtcdCert, err := base64.StdEncoding.DecodeString(calicoConfig.EtcdCert)
	if err != nil {
		klog.Errorf("decoding exception, etcd cert is invalid, cluster name is %s.", cluster.Name)
		return nil, err
	}
	decodeEtcdCACert, err := base64.StdEncoding.DecodeString(calicoConfig.EtcdCACert)
	if err != nil {
		klog.Errorf("decoding exception, etcd ca.cert is invalid, cluster name is %s.", cluster.Name)
		return nil, err
	}

	calicoAPIConfig.Spec.DatastoreType = apiconfig.DatastoreType(calicoConfig.DatastoreType)
	calicoAPIConfig.Spec.EtcdEndpoints = calicoConfig.EtcdEndpoints
	calicoAPIConfig.Spec.EtcdKey = string(decodeEtcdKey)
	calicoAPIConfig.Spec.EtcdCert = string(decodeEtcdCert)
	calicoAPIConfig.Spec.EtcdCACert = string(decodeEtcdCACert)

	calicoClient, err := clientv3.New(*calicoAPIConfig)
	if err != nil {
		klog.Errorf("failed to new kubeClient, cluster name is %s.", cluster.Name)
		return nil, err
	}

	return calicoClient, nil
}

func ResolveServiceCIDRs(pod *corev1.Pod) ([]string, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, fmt.Errorf("no containers found in pod, pod name is %s", pod.Name)
	}

	serviceCIDRS := make([]string, 0, 5)
	for i := range pod.Spec.Containers {
		container := pod.Spec.Containers[i]
		command := container.Command
		for j := range command {
			line := command[j]
			if strings.HasPrefix(line, ServiceClusterIpRange) {
				idx := strings.Index(line, "=")
				serviceIpRange := line[idx+1:]
				serviceCIDRS = strings.Split(serviceIpRange, ",")
			}
		}
	}
	return serviceCIDRS, nil
}
