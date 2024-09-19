package constants

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	InitControllerName               = "virtual-cluster-init-controller"
	NodeControllerName               = "virtual-cluster-node-controller"
	GlobalNodeControllerName         = "global-node-controller"
	KosmosJoinControllerName         = "kosmos-join-controller"
	KosmosNs                         = "kosmos-system"
	SystemNs                         = "kube-system"
	DefaultNs                        = "default"
	DefaultImageRepositoryEnv        = "IMAGE_REPOSITIRY"
	DefaultImageVersionEnv           = "IMAGE_VERSION"
	DefaultCoreDNSImageTagEnv        = "COREDNS_IMAGE_TAG"
	DefaultVirtualControllerLabelEnv = "VIRTUAL_CONTROLLER_LABEL"
	VirtualClusterFinalizerName      = "kosmos.io/virtual-cluster-finalizer"
	ServiceType                      = "NodePort"
	EtcdServiceType                  = "ClusterIP"
	DisableCascadingDeletionLabel    = "operator.virtualcluster.io/disable-cascading-deletion"
	ControllerFinalizerName          = "operator.virtualcluster.io/finalizer"
	DefaultKubeconfigPath            = "/etc/cluster-tree/cert"
	Label                            = "virtualCluster-app"
	ComponentBeReadyTimeout          = 300 * time.Second
	ComponentBeDeletedTimeout        = 300 * time.Second

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
	APIServer                     = "apiserver"
	APIServerAnp                  = "apiserver-anp"
	APIServerEtcdListenClientPort = 2379
	APIServerServiceType          = "NodePort"
	// APIServerCallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	APIServerCallRetryInterval = 100 * time.Millisecond
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
	APIServerPortKey                   = "apiserver-port"
	APIServerNetworkProxyAgentPortKey  = "apiserver-network-proxy-agent-port"
	APIServerNetworkProxyServerPortKey = "apiserver-network-proxy-server-port"
	APIServerNetworkProxyHealthPortKey = "apiserver-network-proxy-health-port"
	APIServerNetworkProxyAdminPortKey  = "apiserver-network-proxy-admin-port"
	VirtualClusterPortNum              = 5

	// vip
	VipPoolConfigMapName        = "kosmos-vip-pool"
	VipPoolKey                  = "vip-config.yaml"
	VcVipStatusKey              = "vip-key"
	VipKeepAlivedNodeLabelKey   = "kosmos.io/keepalived-node"
	VipKeepAlivedNodeLabelValue = "true"
	VipKeepAlivedNodeRoleKey    = "kosmos.io/keepalived-role"
	VipKeepAlivedNodeRoleMaster = "master"
	VipKeepalivedNodeRoleBackup = "backup"
	VipKeepAlivedReplicas       = 3
	VipKeepalivedComponentName  = "keepalived"

	ManifestComponentsConfigMap = "components-manifest-cm"

	WaitAllPodsRunningTimeoutSeconds = 1800

	// core-dns
	KubeDNSSVCName = "kube-dns"
	// nolint
	HostCoreDnsComponents      = "host-core-dns-components"
	VirtualCoreDNSComponents   = "virtual-core-dns-components"
	PrometheusRuleManifest     = "prometheus-rules"
	TenantCoreDNSComponentName = "core-dns-tenant"

	StateLabelKey = "kosmos-io/state"

	KonnectivityServerSuffix = "konnectivity-server"

	//in virtual cluster
	APIServerExternalService = "api-server-external-service"
)

type Action string

var APIServerServiceSubnet string
var KubeControllerManagerPodSubnet string

var PreferredAddressType corev1.NodeAddressType

func init() {
	APIServerServiceSubnet = utils.GetEnvWithDefaultValue("SERVICE_SUBNET", "10.237.6.0/18")
	// fd11:1122:1111::/48,
	KubeControllerManagerPodSubnet = utils.GetEnvWithDefaultValue("POD_SUBNET", "10.244.0.0/16")

	PreferredAddressType = corev1.NodeAddressType(utils.GetEnvWithDefaultValue("PREFERRED_ADDRESS_TYPE", string(corev1.NodeInternalIP)))
}
