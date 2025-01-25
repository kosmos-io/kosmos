package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/projectcalico/calico/libcalico-go/lib/apiconfig"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/etcdv3"
	"github.com/projectcalico/calico/libcalico-go/lib/clientv3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	DataStoreType = "datastoreType"
	EtcdV3        = "etcdv3"

	ServiceClusterIPRange = "--service-cluster-ip-range"
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
	calicoConfig, err := getCalicoAPIConfig(cluster)
	if err != nil {
		klog.Errorf("Error getting calicAPIConfig: %v", err)
		return nil, err
	}

	calicoClient, err := clientv3.New(*calicoConfig)
	if err != nil {
		klog.Errorf("failed to new kubeClient, cluster name is %s.", cluster.Name)
		return nil, err
	}

	return calicoClient, nil
}

func GetETCDClient(cluster *clusterlinkv1alpha1.Cluster) (api.Client, error) {
	calicoConfig, err := getCalicoAPIConfig(cluster)
	if err != nil {
		klog.Errorf("Error getting calicAPIConfig: %v", err)
		return nil, err
	}

	etcdV3Client, err := etcdv3.NewEtcdV3Client(&calicoConfig.Spec.EtcdConfig)
	if err != nil {
		klog.Errorf("failed to new etcdClient, cluster name is %s.", cluster.Name)
		return nil, err
	}

	return etcdV3Client, nil
}

func getCalicoAPIConfig(cluster *clusterlinkv1alpha1.Cluster) (*apiconfig.CalicoAPIConfig, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %v", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build k8s kubeClient: %v", err)
	}

	clusterConfigMap, err := clientSet.CoreV1().ConfigMaps(utils.DefaultNamespace).Get(context.Background(), cluster.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster ConfigMap for cluster %s: %v", cluster.Name, err)
	}

	var calicoConfig CalicoConfig
	calicoData := clusterConfigMap.Data
	calicoJSONStr, err := json.Marshal(calicoData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ConfigMap data for cluster %s to JSON: %v", cluster.Name, err)
	}

	if err := json.Unmarshal(calicoJSONStr, &calicoConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to CalicoConfig for cluster %s: %v", cluster.Name, err)
	}

	etcdKey, err := decodeBase64(calicoConfig.EtcdKey, "etcd key", cluster.Name)
	if err != nil {
		return nil, err
	}
	etcdCert, err := decodeBase64(calicoConfig.EtcdCert, "etcd cert", cluster.Name)
	if err != nil {
		return nil, err
	}
	etcdCACert, err := decodeBase64(calicoConfig.EtcdCACert, "etcd CA cert", cluster.Name)
	if err != nil {
		return nil, err
	}

	calicoAPIConfig := apiconfig.NewCalicoAPIConfig()
	calicoAPIConfig.Spec.DatastoreType = apiconfig.DatastoreType(calicoConfig.DatastoreType)
	calicoAPIConfig.Spec.EtcdEndpoints = calicoConfig.EtcdEndpoints
	calicoAPIConfig.Spec.EtcdKey = etcdKey
	calicoAPIConfig.Spec.EtcdCert = etcdCert
	calicoAPIConfig.Spec.EtcdCACert = etcdCACert

	return calicoAPIConfig, nil
}

func decodeBase64(encoded, fieldName, clusterName string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode %s for cluster %s: %v", fieldName, clusterName, err)
	}
	return string(decoded), nil
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
			if strings.HasPrefix(line, ServiceClusterIPRange) {
				idx := strings.Index(line, "=")
				serviceIPRange := line[idx+1:]
				serviceCIDRS = strings.Split(serviceIPRange, ",")
			}
		}
	}

	for i, cidr := range serviceCIDRS {
		ipNetStr, err := utils.FormatCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to format service cidr %s, pod name is %s, err: %s", cidr, pod.Name, err.Error())
		}
		serviceCIDRS[i] = ipNetStr
	}

	return serviceCIDRS, nil
}
