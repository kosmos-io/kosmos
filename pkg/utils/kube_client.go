package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"

	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

var (
	defaultKubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
)

const (
	DefaultKubeQPS   = 5.0
	DefaultKubeBurst = 10

	DefaultTreeAndNetManagerKubeQPS   = 40.0
	DefaultTreeAndNetManagerKubeBurst = 60
)

type KubernetesOptions struct {
	KubeConfig             string  `json:"kubeconfig" yaml:"kubeconfig"`
	MasterURL              string  `json:"masterURL,omitempty" yaml:"masterURL,omitempty"`
	ControlpanelKubeConfig string  `json:"controlpanelKubeConfig,omitempty" yaml:"controlpanelKubeConfig,omitempty"`
	ControlpanelMasterURL  string  `json:"controlpanelMasterURL,omitempty" yaml:"controlpanelMasterURL,omitempty"`
	QPS                    float32 `json:"qps,omitempty" yaml:"qps,omitempty"`
	Burst                  int     `json:"burst,omitempty" yaml:"burst,omitempty"`
}

func loadKubeconfig(kubeconfigPath, context string) (*clientcmdapi.Config, error) {
	if kubeconfigPath == "" {
		kubeconfigPath = GetEnvString("KUBECONFIG", defaultKubeConfig)
	}

	if _, err := os.Stat(kubeconfigPath); err != nil {
		return nil, fmt.Errorf("kubeconfig path %s does not exist", kubeconfigPath)
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	loadingRules := *pathOptions.LoadingRules
	loadingRules.ExplicitPath = kubeconfigPath
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&loadingRules, overrides)
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// RestConfig creates a rest config from the context and kubeconfig.
func RestConfig(kubeconfigPath, context string) (*rest.Config, error) {
	rawConfig, err := loadKubeconfig(kubeconfigPath, context)
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.NewDefaultClientConfig(*rawConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}

	return restConfig, nil
}

// RawConfig creates a raw config from the context and kubeconfig.
func RawConfig(kubeconfigPath, context string) (clientcmdapi.Config, error) {
	rawConfig, err := loadKubeconfig(kubeconfigPath, context)
	if err != nil {
		return clientcmdapi.Config{}, err
	}

	return *rawConfig, nil
}

// GetEnvString returns the env variable,if the env is not set,return the defaultValue
func GetEnvString(key, defaultValue string) string {
	v, ok := os.LookupEnv(key)
	if ok {
		return v
	}
	return defaultValue
}

type Opts func(*rest.Config)

func NewConfigFromBytes(kubeConfig []byte, opts ...Opts) (*rest.Config, error) {
	var (
		config *rest.Config
		err    error
	)

	c, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = c.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, h := range opts {
		if h == nil {
			continue
		}
		h(config)
	}

	return config, nil
}

func NewClientFromConfigPath(configPath string, opts ...Opts) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func SetQPSBurst(config *rest.Config, options KubernetesOptions) {
	config.QPS = options.QPS
	config.Burst = options.Burst
}

func NewKosmosClientFromConfigPath(configPath string, opts ...Opts) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func NewClientFromBytes(kubeConfig []byte, opts ...Opts) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}

func NewKosmosClientFromBytes(kubeConfig []byte, opts ...Opts) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}
