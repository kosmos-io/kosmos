package backup

import (
	"github.com/pkg/errors"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
)

// SerivceAccountAction implements ItemAction
type ServiceAccountAction struct {
	clusterRoleBindings []ClusterRoleBinding
	fetched             bool
}

func NewServiceAccountAction() *ServiceAccountAction {
	return &ServiceAccountAction{
		fetched: false,
	}
}

func (s *ServiceAccountAction) Resource() string {
	return "serviceaccounts"
}

// Execute checks for any ClusterRoleBindings that have this service account as a subject, and
// adds the ClusterRoleBinding and associated ClusterRole to the list of additional items to
// be backed up.
func (s *ServiceAccountAction) Execute(item runtime.Unstructured, backup *kubernetesBackupper) (runtime.Unstructured, []requests.ResourceIdentifier, error) {
	klog.Info("Running ServiceAccountAction")
	defer klog.Info("Done running ServiceAccountAction")

	if !s.fetched {
		err := s.fetchClusterRoleBindings(backup)
		if err != nil {
			return nil, nil, errors.WithMessage(err, "fetchClusterRoleBindings error")
		}
		s.fetched = true
	}

	objectMeta, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	var (
		namespace = objectMeta.GetNamespace()
		name      = objectMeta.GetName()
		bindings  = sets.NewString()
		roles     = sets.NewString()
	)

	for _, crb := range s.clusterRoleBindings {
		for _, subject := range crb.ServiceAccountSubjects(namespace) {
			if subject == name {
				klog.Infof("Adding clusterrole %s and clusterrolebinding %s to additionalItems since serviceaccount %s/%s is a subject",
					crb.RoleRefName(), crb.Name(), namespace, name)
			}

			bindings.Insert(crb.Name())
			roles.Insert(crb.RoleRefName())
		}
	}

	var additionalItems []requests.ResourceIdentifier
	for binding := range bindings {
		additionalItems = append(additionalItems, requests.ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoleBindings,
			Name:          binding,
		})
	}

	for role := range roles {
		additionalItems = append(additionalItems, requests.ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoles,
			Name:          role,
		})
	}

	return item, additionalItems, nil
}

func (s *ServiceAccountAction) fetchClusterRoleBindings(backup *kubernetesBackupper) error {
	clusterRoleBindingListers := NewClusterRoleBindingListerMap(backup.kubeclient)
	// Look up the supported RBAC version
	var supportedAPI metav1.GroupVersionForDiscovery
	for _, ag := range backup.discoveryHelper.APIGroups() {
		if ag.Name == rbac.GroupName {
			supportedAPI = ag.PreferredVersion
			break
		}
	}

	crbLister := clusterRoleBindingListers[supportedAPI.Version]

	// This should be safe because the List call will return a 0-item slice if there is no matching API version
	crbs, err := crbLister.List()
	if err != nil {
		return err
	}

	s.clusterRoleBindings = crbs
	return nil
}
