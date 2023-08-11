package role

import (
	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func AddRole(node *v1alpha1.ClusterNode, role v1alpha1.Role) {
	if node.Spec.Roles == nil {
		node.Spec.Roles = make([]v1alpha1.Role, 0, 5)
	}
	roleSet := sets.NewString()
	for _, role := range node.Spec.Roles {
		roleSet.Insert(string(role))
	}
	roleSet.Insert(string(role))
	roles := make([]v1alpha1.Role, 0, len(roleSet))
	for _, roleStr := range roleSet.UnsortedList() {
		roles = append(roles, v1alpha1.Role(roleStr))
	}
	node.Spec.Roles = roles
}

func HasRole(node v1alpha1.ClusterNode, role v1alpha1.Role) bool {
	if node.Spec.Roles == nil {
		return false
	}
	for _, r := range node.Spec.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func RemoveRole(node *v1alpha1.ClusterNode, role v1alpha1.Role) {
	if node.Spec.Roles == nil {
		return
	}
	roleSet := sets.NewString()
	for _, role := range node.Spec.Roles {
		roleSet.Insert(string(role))
	}
	roleSet.Delete(string(role))
	roles := make([]v1alpha1.Role, 0, len(roleSet))
	for _, roleStr := range roleSet.UnsortedList() {
		roles = append(roles, v1alpha1.Role(roleStr))
	}
	node.Spec.Roles = roles
}
