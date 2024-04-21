package constants

import "time"

const (
	InitControllerName            = "virtual-cluster-init-controller"
	KosmosJoinControllerName      = "kosmos-join-controller"
	NodePoolControllerName        = "virtualcluster_nodepool_controller"
	SystemNs                      = "kube-system"
	DefauleImageRepositoryEnv     = "IMAGE_REPOSITIRY"
	DefauleImageVersionEnv        = "IMAGE_VERSION"
	VirtualClusterStatusCompleted = "Completed"
	VirtualClusterFinalizerName   = "kosmos.io/virtual-cluster-finalizer"
	ServiceType                   = "NodePort"
	EtcdServiceType               = "ClusterIP"
	DisableCascadingDeletionLabel = "operator.virtualcluster.io/disable-cascading-deletion"
	ControllerFinalizerName       = "operator.virtualcluster.io/finalizer"
	DefaultKubeconfigPath         = "/etc/cluster-tree/cert"
	Label                         = "virtualCluster-app"
	ComponentBeReadyTimeout       = 120 * time.Second

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

	//controlplane apiserver
	ApiServer                     = "apiserver"
	ApiServerReplicas             = 1
	ApiServerServiceSubnet        = "10.237.6.18/29"
	ApiServerEtcdListenClientPort = 2379
	ApiServerServiceType          = "NodePort"
	// APICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	ApiServerCallRetryInterval = 100 * time.Millisecond

	//controlplane etcd
	Etcd                 = "etcd"
	EtcdReplicas         = 3
	EtcdDataVolumeName   = "etcd-data"
	EtcdListenClientPort = 2379
	EtcdListenPeerPort   = 2380

	//controlplane kube-controller
	KubeControllerReplicas         = 1
	KubeControllerManagerComponent = "KubeControllerManager"
	KubeControllerManager          = "kube-controller-manager"

	//controlplane scheduler
	VirtualClusterSchedulerReplicas           = 1
	VirtualClusterSchedulerComponent          = "VirtualClusterScheduler"
	VirtualClusterSchedulerComponentConfigMap = "scheduler-config"
	VirtualClusterScheduler                   = "virtualCluster-scheduler"

	//controlplane auth
	AdminConfig = "admin-config"
	KubeConfig  = "kubeconfig"

	//controlplane upload
	VirtualClusterLabelKeyName = "app.kubernetes.io/managed-by"
	VirtualClusterController   = "virtual-cluster-controller"
	ClusterName                = "virtualCluster-apiserver"
	UserName                   = "virtualCluster-admin"

	// InitAction represents init virtual cluster instance
	InitAction Action = "init"
	// DeInitAction represents delete virtual cluster instance
	DeInitAction Action = "deInit"
)

type Action string
