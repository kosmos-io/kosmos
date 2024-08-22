package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

var (
	// aggregatedScheme aggregates Kubernetes and extended schemes.
	aggregatedScheme = runtime.NewScheme()

	// Codecs provides access to encoding and decoding for the scheme.
	Codecs = serializer.NewCodecFactory(aggregatedScheme, serializer.EnableStrict)
)

func init() {
	err := scheme.AddToScheme(aggregatedScheme) // add Kubernetes schemes
	if err != nil {
		panic(err)
	}
	err = kosmosv1alpha1.AddToScheme(aggregatedScheme) // add clusterlink schemes
	if err != nil {
		panic(err)
	}
	err = mcsv1alpha1.AddToScheme(aggregatedScheme) // add mcs schemes
	if err != nil {
		panic(err)
	}
}

// NewSchema returns a singleton schema set which aggregated Kubernetes's schemes and extended schemes.
func NewSchema() *runtime.Scheme {
	return aggregatedScheme
}
