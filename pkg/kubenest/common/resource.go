package common

import (
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

type APIServerExternalResource struct {
	Namespace     string
	Name          string
	Vc            *v1alpha1.VirtualCluster
	RootClientSet kubernetes.Interface
}
