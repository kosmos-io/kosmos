package constants

import (
	"time"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	InitControllerName               = "virtual-cluster-init-controller"
	NodeControllerName               = "virtual-cluster-node-controller"
	PluginControllerName             = "virtual-cluster-plugin-controller"
	GlobalNodeControllerName         = "global-node-controller"
	KosmosJoinControllerName         = "kosmos-join-controller"
	KosmosNs                         = "kosmos-system"
	SystemNs                         = "kube-system"
	DefaultNs                        = "default"
	DefaultImageRepositoryEnv        = "IMAGE_REPOSITIRY"
	DefaultImageVersionEnv           = "IMAGE_VERSION"
	DefaultCoreDnsImageTagEnv        = "COREDNS_IMAGE_TAG"
	VirtualClusterFinalizerName      = "kosmos.io/virtual-cluster-finalizer"
	ServiceType                      = "NodePort"
	EtcdServiceType                  = "ClusterIP"
	DisableCascadingDeletionLabel    = "operator.virtualcluster.io/disable-cascading-deletion"
	ControllerFinalizerName          = "operator.virtualcluster.io/finalizer"
	DefaultKubeconfigPath            = "/etc/cluster-tree/cert"
	Label                            = "virtualCluster-app"
	ComponentBeReadyTimeout          = 300 * time.Second
	ComponentBeDeletedTimeout        = 300 * time.Second
	DefauleVirtualControllerLabelEnv = "VIRTUAL_CONTROLLER_LABEL"

	// CertificateBlockType is a possible value for pem.Block.Type.
	CertificateBlockType           = "CERTIFICATE"
	RsaKeySize                     = 2048
	KeyExtension                   = ".key"
	CertExtension                  = ".crt"
	CertificateValidity            = time.Hour * 24 * 365
	CaCertAndKeyName               = "ca"
	VirtualClusterCertAndKeyName   = "virtualCluster"
	VirtualClusterSystemNamespace  = "virtualCluster-system"
	ApiserverCertAndKeyName        = "apiserver"
	EtcdCaCertAndKeyName           = "etcd-ca"
	EtcdServerCertAndKeyName       = "etcd-server"
	EtcdClientCertAndKeyName       = "etcd-client"
	FrontProxyCaCertAndKeyName     = "front-proxy-ca"
	FrontProxyClientCertAndKeyName = "front-proxy-client"
	ProxyServerCertAndKeyName      = "proxy-server"

	//controlplane apiserver
	ApiServer                     = "apiserver"
	ApiServerAnp                  = "apiserver-anp"
	ApiServerEtcdListenClientPort = 2379
	ApiServerServiceType          = "NodePort"
	// APICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	ApiServerCallRetryInterval = 100 * time.Millisecond
	APIServerSVCPortName       = "client"

	//install kube-proxy in virtualCluster
	Proxy = "kube-proxy"
	// configmap kube-proxy clustercidr

	//controlplane etcd
	Etcd                 = "etcd"
	EtcdReplicas         = 3
	EtcdDataVolumeName   = "etcd-data"
	EtcdListenClientPort = 2379
	EtcdListenPeerPort   = 2380
	EtcdSuffix           = "-etcd-client"

	//controlplane kube-controller
	KubeControllerReplicas           = 2
	KubeControllerManagerComponent   = "KubeControllerManager"
	KubeControllerManager            = "kube-controller-manager"
	KubeControllerManagerClusterCIDR = "10.244.0.0/16"

	//controlplane scheduler
	VirtualClusterSchedulerReplicas           = 2
	VirtualClusterSchedulerComponent          = "VirtualClusterScheduler"
	VirtualClusterSchedulerComponentConfigMap = "scheduler-config"
	VirtualClusterScheduler                   = "scheduler"
	VirtualClusterKubeProxyComponent          = "kube-proxy"

	//controlplane auth
	AdminConfig        = "admin-config"
	KubeConfig         = "kubeconfig"
	KubeProxyConfigmap = "kube-proxy"

	//controlplane upload
	VirtualClusterLabelKeyName = "app.kubernetes.io/managed-by"
	VirtualClusterController   = "virtual-cluster-controller"
	ClusterName                = "virtualCluster-apiserver"
	UserName                   = "virtualCluster-admin"

	// InitAction represents init virtual cluster instance
	InitAction Action = "init"
	// DeInitAction represents delete virtual cluster instance
	DeInitAction Action = "deInit"

	//host_port_manager
	HostPortsCMName                    = "kosmos-hostports"
	HostPortsCMDataName                = "config.yaml"
	ApiServerPortKey                   = "apiserver-port"
	ApiServerNetworkProxyAgentPortKey  = "apiserver-network-proxy-agent-port"
	ApiServerNetworkProxyServerPortKey = "apiserver-network-proxy-server-port"
	ApiServerNetworkProxyHealthPortKey = "apiserver-network-proxy-health-port"
	ApiServerNetworkProxyAdminPortKey  = "apiserver-network-proxy-admin-port"
	VirtualClusterPortNum              = 5

	ManifestComponentsConfigMap = "components-manifest-cm"

	WaitAllPodsRunningTimeoutSeconds = 1800

	// core-dns
	KubeDNSSVCName = "kube-dns"
	// nolint
	HostCoreDnsComponents    = "host-core-dns-components"
	VirtualCoreDnsComponents = "virtual-core-dns-components"
	PrometheusRuleManifest   = "prometheus-rules"

	StateLabelKey = "kosmos-io/state"

	KonnectivityServerSuffix = "konnectivity-server"

	//in virtual cluster
	ApiServerExternalService = "api-server-external-service"
)

type Action string

var ApiServerServiceSubnet string
var KubeControllerManagerPodSubnet string

func init() {
	ApiServerServiceSubnet = utils.GetEnvWithDefaultValue("SERVICE_SUBNET", "10.237.6.0/18")
	// fd11:1122:1111::/48,
	KubeControllerManagerPodSubnet = utils.GetEnvWithDefaultValue("POD_SUBNET", "10.244.0.0/16")
}
