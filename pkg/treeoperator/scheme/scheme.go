package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(scheme.AddToScheme(Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(Scheme))
}
