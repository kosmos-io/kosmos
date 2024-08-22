package kubenest

import (
	"errors"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/kubenest/tasks"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util/cert"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var _ tasks.InitData = &initData{}

type initData struct {
	cert.CertStore
	name                  string
	namespace             string
	virtualClusterVersion *utilversion.Version
	controlplaneAddr      string
	clusterIps            []string
	remoteClient          clientset.Interface
	kosmosClient          versioned.Interface
	dynamicClient         *dynamic.DynamicClient
	virtualClusterDataDir string
	privateRegistry       string
	externalIP            string
	externalIps           []string
	vipMap                map[string]string
	hostPort              int32
	hostPortMap           map[string]int32
	kubeNestOptions       *v1alpha1.KubeNestConfiguration
	virtualCluster        *v1alpha1.VirtualCluster
	ETCDStorageClass      string
	ETCDUnitSize          string
}

type InitOptions struct {
	Name                  string
	Namespace             string
	Kubeconfig            *rest.Config
	virtualClusterVersion string
	virtualClusterDataDir string
	virtualCluster        *v1alpha1.VirtualCluster
	KubeNestOptions       *v1alpha1.KubeNestConfiguration
}

func NewInitPhase(opts *InitOptions) *workflow.Phase {
	initPhase := workflow.NewPhase()

	initPhase.AppendTask(tasks.NewVirtualClusterServiceTask())
	initPhase.AppendTask(tasks.NewCertTask())
	initPhase.AppendTask(tasks.NewUploadCertsTask())
	initPhase.AppendTask(tasks.NewEtcdTask())
	initPhase.AppendTask(tasks.NewVirtualClusterApiserverTask())
	initPhase.AppendTask(tasks.NewUploadKubeconfigTask())
	initPhase.AppendTask(tasks.NewCheckApiserverHealthTask())
	initPhase.AppendTask(tasks.NewComponentTask())
	initPhase.AppendTask(tasks.NewCheckControlPlaneTask())
	initPhase.AppendTask(tasks.NewAnpTask())
	// create proxy
	//initPhase.AppendTask(tasks.NewVirtualClusterProxyTask())
	// create core-dns
	initPhase.AppendTask(tasks.NewCoreDNSTask())
	// add server
	initPhase.AppendTask(tasks.NewComponentsFromManifestsTask())
	initPhase.AppendTask(tasks.NewEndPointTask())

	initPhase.SetDataInitializer(func() (workflow.RunData, error) {
		return newRunData(opts)
	})
	return initPhase
}

func UninstallPhase(opts *InitOptions) *workflow.Phase {
	destroyPhase := workflow.NewPhase()
	destroyPhase.AppendTask(tasks.UninstallCoreDNSTask())
	destroyPhase.AppendTask(tasks.UninstallComponentTask())
	destroyPhase.AppendTask(tasks.UninstallVirtualClusterApiserverTask())
	// destroyPhase.AppendTask(tasks.UninstallAnpTask())
	destroyPhase.AppendTask(tasks.UninstallEtcdTask())
	destroyPhase.AppendTask(tasks.UninstallVirtualClusterServiceTask())
	destroyPhase.AppendTask(tasks.UninstallCertsAndKubeconfigTask())
	destroyPhase.AppendTask(tasks.DeleteEtcdPvcTask())
	destroyPhase.AppendTask(tasks.UninstallVirtualClusterProxyTask())

	destroyPhase.SetDataInitializer(func() (workflow.RunData, error) {
		return newRunData(opts)
	})
	return destroyPhase
}

type InitOpt func(o *InitOptions)

func NewPhaseInitOptions(opts ...InitOpt) *InitOptions {
	options := defaultJobInitOptions()

	for _, c := range opts {
		c(options)
	}
	return options
}

func defaultJobInitOptions() *InitOptions {
	virtualCluster := &v1alpha1.VirtualCluster{}
	return &InitOptions{
		virtualClusterVersion: "0.0.0",
		virtualClusterDataDir: "var/lib/virtualCluster",
		virtualCluster:        virtualCluster,
	}
}

func NewInitOptWithVirtualCluster(virtualCluster *v1alpha1.VirtualCluster) InitOpt {
	return func(o *InitOptions) {
		o.virtualCluster = virtualCluster
		o.Name = virtualCluster.GetName()
		o.Namespace = virtualCluster.GetNamespace()
	}
}

func NewInitOptWithKubeconfig(config *rest.Config) InitOpt {
	return func(o *InitOptions) {
		o.Kubeconfig = config
	}
}

func NewInitOptWithKubeNestOptions(options *v1alpha1.KubeNestConfiguration) InitOpt {
	return func(o *InitOptions) {
		o.KubeNestOptions = options
	}
}

func newRunData(opt *InitOptions) (*initData, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	localClusterClient, err := clientset.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error when creating local cluster client, err: %w", err)
	}
	var remoteClient clientset.Interface = localClusterClient

	dynamicClient, err := dynamic.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, err
	}

	kosmosClient, err := versioned.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error when creating  kosmosClient client, err: %w", err)
	}

	version, err := utilversion.ParseGeneric(opt.virtualClusterVersion)
	if err != nil {
		return nil, fmt.Errorf("unexpected virtual cluster invalid version %s", opt.virtualClusterVersion)
	}

	var address string
	address, err = util.GetAPIServiceIP(remoteClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get a valid node IP for APIServer, err: %w", err)
	}
	var clusterIps []string
	err, clusterIp := util.GetAPIServiceClusterIp(opt.Namespace, remoteClient)
	clusterIps = append(clusterIps, clusterIp)
	if err != nil {
		return nil, fmt.Errorf("failed to get APIServer Service-ClusterIp, err: %w", err)
	}
	return &initData{
		name:                  opt.Name,
		namespace:             opt.Namespace,
		virtualClusterVersion: version,
		controlplaneAddr:      address,
		clusterIps:            clusterIps,
		remoteClient:          remoteClient,
		dynamicClient:         dynamicClient,
		kosmosClient:          kosmosClient,
		virtualClusterDataDir: opt.virtualClusterDataDir,
		privateRegistry:       utils.DefaultImageRepository,
		CertStore:             cert.NewCertStore(),
		externalIP:            opt.virtualCluster.Spec.ExternalIP,
		externalIps:           opt.virtualCluster.Spec.ExternalIps,
		hostPort:              opt.virtualCluster.Status.Port,
		hostPortMap:           opt.virtualCluster.Status.PortMap,
		vipMap:                opt.virtualCluster.Status.VipMap,
		kubeNestOptions:       opt.KubeNestOptions,
		virtualCluster:        opt.virtualCluster,
		ETCDUnitSize:          opt.KubeNestOptions.KubeInKubeConfig.ETCDUnitSize,
		ETCDStorageClass:      opt.KubeNestOptions.KubeInKubeConfig.ETCDStorageClass,
	}, nil
}

// TODO Add more detailed verification content
func (opt *InitOptions) Validate() error {
	var errs []error

	if len(opt.Name) == 0 || len(opt.Namespace) == 0 {
		return errors.New("unexpected empty name or namespace")
	}

	_, err := utilversion.ParseGeneric(opt.virtualClusterVersion)
	if err != nil {
		return fmt.Errorf("unexpected virtual cluster invalid version %s", opt.virtualClusterVersion)
	}

	return utilerrors.NewAggregate(errs)
}

func (i initData) GetName() string {
	return i.name
}

func (i initData) GetNamespace() string {
	return i.namespace
}

func (i initData) ControlplaneAddress() string {
	return i.controlplaneAddr
}

func (i initData) ServiceClusterIp() []string {
	err, clusterIps := util.GetServiceClusterIp(i.namespace, i.remoteClient)
	if err != nil {
		return nil
	}
	return clusterIps
}

func (i initData) RemoteClient() clientset.Interface {
	return i.remoteClient
}

func (i initData) KosmosClient() versioned.Interface {
	return i.kosmosClient
}

func (i initData) DataDir() string {
	return i.virtualClusterDataDir
}

func (i initData) VirtualCluster() *v1alpha1.VirtualCluster {
	return i.virtualCluster
}

func (i initData) ExternalIP() string {
	return i.externalIP
}

func (i initData) ExternalIPs() []string { return i.externalIps }

func (i initData) VipMap() map[string]string {
	return i.vipMap
}
func (i initData) HostPort() int32 {
	return i.hostPort
}

func (i initData) HostPortMap() map[string]int32 {
	return i.hostPortMap
}

func (i initData) DynamicClient() *dynamic.DynamicClient {
	return i.dynamicClient
}

func (i initData) KubeNestOpt() *v1alpha1.KubeNestConfiguration {
	return i.kubeNestOptions
}

func (i initData) PluginOptions() map[string]string {
	if i.virtualCluster.Spec.PluginOptions == nil {
		return nil
	}

	pluginOptoinsMapping := map[string]string{}

	for _, option := range i.virtualCluster.Spec.PluginOptions {
		pluginOptoinsMapping[option.Name] = option.Value
	}
	return pluginOptoinsMapping
}
