package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

// aggregatedScheme aggregates Kubernetes and extended schemes.
var aggregatedScheme = runtime.NewScheme()

func init() {
	err := scheme.AddToScheme(aggregatedScheme) // add Kubernetes schemes
	if err != nil {
		panic(err)
	}
	err = clusterlinkv1alpha1.AddToScheme(aggregatedScheme) // add clusterlink schemes
	if err != nil {
		panic(err)
	}
}

// NewSchema returns a singleton schema set which aggregated Kubernetes's schemes and extended schemes.
func NewSchema() *runtime.Scheme {
	return aggregatedScheme
}

// NewForConfig creates a new client for the given config.
func NewForConfig(config *rest.Config) (client.Client, error) {
	return client.New(config, client.Options{
		Scheme: aggregatedScheme,
	})
}

// NewForConfigOrDie creates a new client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(config *rest.Config) client.Client {
	c, err := NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return c
}
