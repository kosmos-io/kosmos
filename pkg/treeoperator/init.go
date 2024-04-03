package treeoperator

import (
	"errors"
	"fmt"
	"sync"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/cert"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/scheme"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/tasks"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/util"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

var _ tasks.InitData = &initData{}
var (
	DefaultKarmadaImageVersion string
)

const (
	PrivateRegistry = "registry.paas"
)

type initData struct {
	sync.Once
	cert.CertStore
	name                  string
	namespace             string
	virtualClusterVersion *utilversion.Version
	controlplaneConfig    *rest.Config
	controlplaneAddr      string
	remoteClient          clientset.Interface
	virtualClusterClient  clientset.Interface
	kosmosClient          versioned.Interface
	dnsDomain             string
	virtualClusterDataDir string
	privateRegistry       string
}

type InitOptions struct {
	Name                  string
	Namespace             string
	Kubeconfig            *rest.Config
	virtualClusterVersion string
	virtualClusterDataDir string
	virtualCluster        *v1alpha1.VirtualCluster
}

func NewInitPhase(opts *InitOptions) *workflow.Phase {
	initPhase := workflow.NewPhase()

	initPhase.AppendTask(tasks.NewCertTask())
	initPhase.AppendTask(tasks.NewUploadCertsTask())
	initPhase.AppendTask(tasks.NewEtcdTask())
	initPhase.AppendTask(tasks.NewVirtualClusterApiserverTask())
	initPhase.AppendTask(tasks.NewUploadKubeconfigTask())
	initPhase.AppendTask(tasks.NewCheckApiserverHealthTask())
	initPhase.AppendTask(tasks.NewComponentTask())
	//initPhase.AppendTask(tasks.NewRBACTask())
	initPhase.AppendTask(tasks.NewCheckControlPlaneTask())
	//initPhase.AppendTask(tasks.NewUpdateVirtualClusterObjectTask())

	initPhase.SetDataInitializer(func() (workflow.RunData, error) {
		return newRunData(opts)
	})
	return initPhase
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

	scheme.Scheme.Default(virtualCluster)

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

func newRunData(opt *InitOptions) (*initData, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	localClusterClient, err := clientset.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error when creating local cluster client, err: %w", err)
	}
	var remoteClient clientset.Interface
	remoteClient = localClusterClient

	kosmosClient, err := versioned.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error when creating  kosmosClient client, err: %w", err)
	}

	var privateRegistry string
	privateRegistry = PrivateRegistry

	version, err := utilversion.ParseGeneric(opt.virtualClusterVersion)
	if err != nil {
		return nil, fmt.Errorf("unexpected virtual cluster invalid version %s", opt.virtualClusterVersion)
	}

	var address string
	address, err = util.GetAPIServiceIP(remoteClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get a valid node IP for APIServer, err: %w", err)
	}

	return &initData{
		name:                  opt.Name,
		namespace:             opt.Namespace,
		virtualClusterVersion: version,
		controlplaneAddr:      address,
		remoteClient:          remoteClient,
		kosmosClient:          kosmosClient,
		virtualClusterDataDir: opt.virtualClusterDataDir,
		privateRegistry:       privateRegistry,
		CertStore:             cert.NewCertStore(),
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

func (i initData) SetControlplaneConfig(config *rest.Config) {
	i.controlplaneConfig = config
}

func (i initData) ControlplaneConfig() *rest.Config {
	return i.controlplaneConfig
}

func (i initData) ControlplaneAddress() string {
	return i.controlplaneAddr
}

func (i initData) RemoteClient() clientset.Interface {
	return i.remoteClient
}

func (i initData) VirtualClusterClient() clientset.Interface {
	if i.virtualClusterClient == nil {
		i.Once.Do(func() {
			client, err := clientset.NewForConfig(i.controlplaneConfig)
			if err != nil {
				klog.Errorf("error when init virtual cluster client, err: %w", err)
			}
			i.virtualClusterClient = client
		})
	}

	return i.virtualClusterClient
}

func (i initData) KosmosClient() versioned.Interface {
	return i.kosmosClient
}

func (i initData) DataDir() string {
	return i.virtualClusterDataDir
}

func (i initData) VirtualClusterVersion() string {
	return i.virtualClusterVersion.String()
}
