package options

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	genericserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/kosmos.io/kosmos/pkg/apis/proxy/scheme"
	proxyScheme "github.com/kosmos.io/kosmos/pkg/apis/proxy/scheme"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy"
	proxyctl "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/controller"
	kosmosclientset "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	informerfactory "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/kosmos.io/kosmos/pkg/generated/openapi"
	profileflag "github.com/kosmos.io/kosmos/pkg/sharedcli/profileflag"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Options struct {
	// RecommendedOptions *genericoptions.RecommendedOptions
	GenericServerRunOptions *genericoptions.ServerRunOptions
	SecureServing           *genericoptions.SecureServingOptionsWithLoopback
	Authentication          *genericoptions.DelegatingAuthenticationOptions
	Authorization           *genericoptions.DelegatingAuthorizationOptions
	Audit                   *genericoptions.AuditOptions
	Features                *genericoptions.FeatureOptions
	CoreAPI                 *genericoptions.CoreAPIOptions
	ServerRunOptions        *genericoptions.ServerRunOptions

	ProfileOpts profileflag.Options
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
	o.CoreAPI.AddFlags(flags)
	o.ServerRunOptions.AddUniversalFlags(flags)
	o.ProfileOpts.AddFlags(flags)
}

// nolint
func NewOptions() *Options {
	o := &Options{
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		SecureServing:           genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication:          genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:           genericoptions.NewDelegatingAuthorizationOptions(),
		Audit:                   genericoptions.NewAuditOptions(),
		Features:                genericoptions.NewFeatureOptions(),
		CoreAPI:                 genericoptions.NewCoreAPIOptions(),
		ServerRunOptions:        genericoptions.NewServerRunOptions(),
	}
	return o
}

// nolint
func (o *Options) Validate() error {
	errs := []error{}
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Audit.Validate()...)
	errs = append(errs, o.Features.Validate()...)
	return utilerrors.NewAggregate(errs)
}

// nolint
func (o *Options) Config() (*proxy.Config, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}

	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error create self-signed certificates: %v", err)
	}

	// o.Admission.DisablePlugins = append(o.RecommendedOptions.Admission.DisablePlugins, lifecycle.PluginName)

	genericConfig := genericserver.NewRecommendedConfig(proxyScheme.Codecs)
	genericConfig.OpenAPIConfig = genericserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(scheme.Scheme))
	genericConfig.OpenAPIConfig.Info.Title = utils.KosmosClusrerLinkRroxyComponentName
	genericConfig.OpenAPIConfig.Info.Version = utils.ClusterLinkOpenAPIVersion

	// support watch to LongRunningFunc
	genericConfig.LongRunningFunc = func(r *http.Request, requestInfo *genericrequest.RequestInfo) bool {
		return strings.Contains(r.RequestURI, "watch")
	}

	if err := o.ApplyTo(genericConfig); err != nil {
		return nil, err
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(genericConfig.ClientConfig)
	if err != nil {
		klog.Errorf("Failed to create REST mapper: %v", err)
		return nil, err
	}
	kosmosClient := kosmosclientset.NewForConfigOrDie(genericConfig.ClientConfig)
	kosmosInformerFactory := informerfactory.NewSharedInformerFactory(kosmosClient, 0)

	dynamicClient, err := dynamic.NewForConfig(genericConfig.ClientConfig)
	if err != nil {
		log.Fatal(err)
	}

	proxyCtl, err := proxyctl.NewResourceCacheController(proxyctl.NewControllerOption{
		RestConfig:    genericConfig.ClientConfig,
		RestMapper:    restMapper,
		KosmosFactory: kosmosInformerFactory,
		DynamicClient: dynamicClient,
	})
	if err != nil {
		return nil, err
	}

	return &proxy.Config{
		GenericConfig: genericConfig,
		ExtraConfig: proxy.ExtraConfig{
			ProxyController:       proxyCtl,
			KosmosInformerFactory: kosmosInformerFactory,
		},
	}, nil
}

func (o *Options) ApplyTo(config *genericserver.RecommendedConfig) error {
	if err := o.SecureServing.ApplyTo(&config.Config.SecureServing, &config.Config.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Authentication.ApplyTo(&config.Config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Authorization.ApplyTo(&config.Config.Authorization); err != nil {
		return err
	}
	if err := o.Audit.ApplyTo(&config.Config); err != nil {
		return err
	}

	if err := o.Features.ApplyTo(&config.Config); err != nil {
		return err
	}
	if err := o.CoreAPI.ApplyTo(config); err != nil {
		return err
	}
	if err := o.ServerRunOptions.ApplyTo(&config.Config); err != nil {
		return err
	}
	return o.Features.ApplyTo(&config.Config)
}
