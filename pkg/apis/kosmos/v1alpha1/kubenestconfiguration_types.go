package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeNestType string

const (
	KubeInKube KubeNestType = "Kube in kube"
	KosmosKube KubeNestType = "Kosmos in kube"
)

type APIServerServiceType string

const (
	HostNetwork APIServerServiceType = "hostNetwork"
	NodePort    APIServerServiceType = "nodePort"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeNestConfiguration defines the configuration for KubeNest
type KubeNestConfiguration struct {
	// TypeMeta contains the API version and kind.
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	KubeNestType KubeNestType `yaml:"kubeNestType" json:"kubeNestType,omitempty"`

	KosmosKubeConfig KosmosKubeConfig `yaml:"kosmosKubeConfig" json:"kosmosKubeConfig,omitempty"`

	KubeInKubeConfig KubeInKubeConfig `yaml:"kubeInKubeConfig" json:"kubeInKubeConfig,omitempty"`
}

type EtcdCluster struct {
}

type KosmosKubeConfig struct {
	// AllowNodeOwnbyMulticluster indicates whether to allow nodes to be owned by multiple clusters.
	AllowNodeOwnbyMulticluster bool `yaml:"allowNodeOwnbyMulticluster" json:"allowNodeOwnbyMulticluster,omitempty"`
}

type KubeInKubeConfig struct {
	// todo Group according to the parameters of apiserver, etcd, coredns, etc.
	// ForceDestroy indicates whether to force destroy the cluster.
	// +optional
	ForceDestroy bool `yaml:"forceDestroy" json:"forceDestroy,omitempty"`
	// +optional
	AnpMode string `yaml:"anpMode" json:"anpMode,omitempty"`
	// +optional
	AdmissionPlugins bool `yaml:"admissionPlugins" json:"admissionPlugins,omitempty"`
	// +optional
	APIServerReplicas int `yaml:"apiServerReplicas" json:"apiServerReplicas,omitempty"`
	// +optional
	ClusterCIDR string `yaml:"clusterCIDR" json:"clusterCIDR,omitempty"`
	// +optional
	ETCDStorageClass string `yaml:"etcdStorageClass" json:"etcdStorageClass,omitempty"`
	// +optional
	ETCDUnitSize string `yaml:"etcdUnitSize" json:"etcdUnitSize,omitempty"`

	//// Etcd contains the configuration for the etcd statefulset.
	//Etcd EtcdCluster `yaml:"etcd" json:"etcd,omitempty"`

	//// DNS  contains the configuration for the dns server in kubernetes.
	//DNS DNS `yaml:"dns" json:"dns,omitempty"`
	//
	//// Kubernetes contains the configuration for the kubernetes.
	//Kubernetes Kubernetes `yaml:"kubernetes" json:"kubernetes,omitempty"`
	//
	//// Network  contains the configuration for the network in kubernetes cluster.
	//Network NetworkConfig `yaml:"network" json:"network,omitempty"`
	//
	//// Storage contains the configuration for the storage in kubernetes cluster.
	//Storage StorageConfig `yaml:"storage" json:"storage,omitempty"`
	//
	//// Registry contains the configuration for the registry in kubernetes cluster.
	//Registry RegistryConfig `yaml:"registry" json:"registry,omitempty"`

	//TenantEntrypoint TenantEntrypoint `yaml:"tenantEntrypoint" json:"tenantEntrypoint,omitempty"`
	// +optional
	TenantEntrypoint TenantEntrypoint `yaml:"tenantEntrypoint" json:"tenantEntrypoint,omitempty"`

	// +kubebuilder:validation:Enum=nodePort;hostNetwork
	// +kubebuilder:default=hostNetwork
	// +optional
	APIServerServiceType APIServerServiceType `yaml:"apiServerServiceType" json:"apiServerServiceType,omitempty"`

	// +kubebuilder:default=false
	// +optional
	UseTenantDNS bool `yaml:"useTenantDNS" json:"useTenantDNS,omitempty"`
	// +optional
	ExternalPort int32 `json:"externalPort,omitempty"`
	// +kubebuilder:default=false
	// +optional
	UseNodeLocalDNS bool `yaml:"useNodeLocalDNS" json:"useNodeLocalDNS,omitempty"`
}

// TenantEntrypoint contains the configuration for the tenant entrypoint.
type TenantEntrypoint struct {
	// ExternalIP is the external ip of the tenant entrypoint.
	// +optional
	ExternalIps []string `json:"externalIps,omitempty"`
	// ExternalVips is the external vips of the tenant entrypoint.
	// +optional
	ExternalVips []string `json:"externalVips,omitempty"`
}
