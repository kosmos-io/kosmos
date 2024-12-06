package webhooks

import (
	ctrl "sigs.k8s.io/controller-runtime"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/webhooks/validators"
)

// SetupWebhookWithManager sets up the webhook with the manager
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.VirtualCluster{}).
		WithValidator(&validators.VirtualClusterValidator{}). // Updated validator
		Complete()
}
