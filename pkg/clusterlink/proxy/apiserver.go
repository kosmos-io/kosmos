package proxy

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	clientrest "k8s.io/client-go/rest"

	proxyScheme "github.com/kosmos.io/kosmos/pkg/apis/proxy/scheme"
	"github.com/kosmos.io/kosmos/pkg/apis/proxy/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/controller"
	informerfactory "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	clusterLinkStorage "github.com/kosmos.io/kosmos/pkg/registry/clusterlink/storage"
)

// Config defines the config for the APIServer.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type ExtraConfig struct {
	ProxyController       *controller.ResourceCacheController
	KosmosInformerFactory informerfactory.SharedInformerFactory
}

// APIServer defines the api server
type APIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
	ClientConfig  *clientrest.Config
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		GenericConfig: cfg.GenericConfig.Complete(),
		ExtraConfig:   &cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

func (c completedConfig) New() (*APIServer, error) {
	genericServer, err := c.GenericConfig.New("clusterlink-proxy-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		v1alpha1.GroupName,
		proxyScheme.Scheme,
		proxyScheme.ParameterCodec,
		proxyScheme.Codecs,
	)

	v1alpha1storage := map[string]rest.Storage{}

	v1alpha1storage["proxying"] = clusterLinkStorage.NewProxyREST(c.ExtraConfig.ProxyController)
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1storage

	if err = server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}
	return server, nil
}
