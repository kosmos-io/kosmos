package options

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/feature"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"

	"github.com/kosmos.io/clusterlink/pkg/proxy"
)

// Options contains command line parameters for clusterlink-proxy
type Options struct {
	MaxRequestsInFlight         int
	MaxMutatingRequestsInFlight int

	Logs           *logs.Options
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Audit          *genericoptions.AuditOptions
	Features       *genericoptions.FeatureOptions
	CoreAPI        *genericoptions.CoreAPIOptions
	FeatureGate    featuregate.FeatureGate
	Admission      *genericoptions.AdmissionOptions
}

// nolint
func NewOptions() *Options {
	sso := genericoptions.NewSecureServingOptions()

	// We are composing recommended options for an aggregated api-server,
	// whose client is typically a proxy multiplexing many operations ---
	// notably including long-running ones --- into one HTTP/2 connection
	// into this server.  So allow many concurrent operations.
	sso.HTTP2MaxStreamsPerConnection = 1000

	return &Options{
		MaxRequestsInFlight:         0,
		MaxMutatingRequestsInFlight: 0,

		Logs:           logs.NewOptions(),
		SecureServing:  sso.WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Audit:          genericoptions.NewAuditOptions(),
		Features:       genericoptions.NewFeatureOptions(),
		CoreAPI:        genericoptions.NewCoreAPIOptions(),
		FeatureGate:    feature.DefaultFeatureGate,
		Admission:      genericoptions.NewAdmissionOptions(),
	}
}

// nolint
func (o *Options) Validate() error {
	errors := []error{}
	errors = append(errors, o.validateGenericOptions()...)
	return utilerrors.NewAggregate(errors)
}

func (o *Options) validateGenericOptions() []error {
	errors := []error{}
	if o.MaxRequestsInFlight < 0 {
		errors = append(errors, fmt.Errorf("--max-requests-inflight can not be negative value"))
	}
	if o.MaxMutatingRequestsInFlight < 0 {
		errors = append(errors, fmt.Errorf("--max-mutating-requests-inflight can not be negative value"))
	}

	errors = append(errors, o.CoreAPI.Validate()...)
	errors = append(errors, o.SecureServing.Validate()...)
	errors = append(errors, o.Authentication.Validate()...)
	errors = append(errors, o.Authorization.Validate()...)
	errors = append(errors, o.Audit.Validate()...)
	errors = append(errors, o.Features.Validate()...)
	return errors
}

// nolint
func (o *Options) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets

	genericfs := fss.FlagSet("generic")
	genericfs.IntVar(&o.MaxRequestsInFlight, "max-requests-inflight", o.MaxRequestsInFlight, ""+
		"Otherwise, this flag limits the maximum number of non-mutating requests in flight, or a zero value disables the limit completely.")
	genericfs.IntVar(&o.MaxMutatingRequestsInFlight, "max-mutating-requests-inflight", o.MaxMutatingRequestsInFlight, ""+
		"this flag limits the maximum number of mutating requests in flight, or a zero value disables the limit completely.")

	o.CoreAPI.AddFlags(fss.FlagSet("global"))
	o.SecureServing.AddFlags(fss.FlagSet("secure serving"))
	o.Authentication.AddFlags(fss.FlagSet("authentication"))
	o.Authorization.AddFlags(fss.FlagSet("authorization"))
	o.Audit.AddFlags(fss.FlagSet("auditing"))
	o.Features.AddFlags(fss.FlagSet("features"))
	logsapi.AddFlags(o.Logs, fss.FlagSet("logs"))

	// o.Admission.AddFlags(fss.FlagSet("admission"))
	// o.Traces.AddFlags(fss.FlagSet("traces"))

	return fss
}

// nolint
func (o *Options) Config() (*proxy.Config, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}

	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error create self-signed certificates: %v", err)
	}

	// remove NamespaceLifecycle admission plugin explicitly
	// current admission plugins:  mutatingwebhook, validatingwebhook
	o.Admission.DisablePlugins = append(o.Admission.DisablePlugins, lifecycle.PluginName)

	genericConfig := genericapiserver.NewRecommendedConfig(proxy.Codecs)
	// genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(apiserver.Scheme))
	// genericConfig.OpenAPIConfig.Info.Title = openAPITitle
	// genericConfig.OpenAPIConfig.Info.Version= openAPIVersion

	// support watch to LongRunningFunc
	genericConfig.LongRunningFunc = func(r *http.Request, requestInfo *genericrequest.RequestInfo) bool {
		return strings.Contains(r.RequestURI, "watch")
	}

	if err := o.genericOptionsApplyTo(genericConfig); err != nil {
		return nil, err
	}

	return &proxy.Config{
		GenericConfig: genericConfig,
	}, nil
}

func (o *Options) genericOptionsApplyTo(config *genericapiserver.RecommendedConfig) error {
	config.MaxRequestsInFlight = o.MaxRequestsInFlight
	config.MaxMutatingRequestsInFlight = o.MaxMutatingRequestsInFlight

	if err := o.SecureServing.ApplyTo(&config.SecureServing, &config.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Authentication.ApplyTo(&config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Authorization.ApplyTo(&config.Authorization); err != nil {
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
	if err := o.Admission.ApplyTo(&config.Config, config.SharedInformerFactory, config.ClientConfig, o.FeatureGate); err != nil {
		return err
	}

	return nil
}
