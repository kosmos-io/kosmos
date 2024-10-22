// nolint:dupl
package role

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

// TestAddRole tests the AddRole function.
func TestAddRole(t *testing.T) {
	tests := []struct {
		name     string
		initial  []v1alpha1.Role
		newRole  v1alpha1.Role
		expected []v1alpha1.Role
	}{
		{
			name:    "Add new role to empty list",
			initial: nil,
			newRole: "new-role",
			expected: []v1alpha1.Role{
				"new-role",
			},
		},
		{
			name: "Add new role to existing roles",
			initial: []v1alpha1.Role{
				"role1",
				"role2",
			},
			newRole: "role3",
			expected: []v1alpha1.Role{
				"role1",
				"role2",
				"role3"},
		},
		{
			name: "Add duplicate role",
			initial: []v1alpha1.Role{
				"role1",
				"role2",
			},
			newRole: "role1",
			expected: []v1alpha1.Role{
				"role1",
				"role2",
			},
		},
		{
			name: "Add role to existing roles with duplicates",
			initial: []v1alpha1.Role{
				"role1",
				"role1",
			},
			newRole: "role2",
			expected: []v1alpha1.Role{
				"role1",
				"role2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1alpha1.ClusterNode{
				Spec: v1alpha1.ClusterNodeSpec{
					Roles: tt.initial,
				},
			}
			AddRole(node, tt.newRole)

			if len(node.Spec.Roles) != len(tt.expected) {
				t.Errorf("expected %d roles, got %d", len(tt.expected), len(node.Spec.Roles))
			}

			roleSet := sets.NewString()
			for _, role := range node.Spec.Roles {
				roleSet.Insert(string(role))
			}

			for _, expectedRole := range tt.expected {
				if !roleSet.Has(string(expectedRole)) {
					t.Errorf("expected role %s not found", expectedRole)
				}
			}
		})
	}
}

// TestHasRole tests the HasRole function.
func TestHasRole(t *testing.T) {
	tests := []struct {
		name      string
		roles     []v1alpha1.Role
		checkRole v1alpha1.Role
		expected  bool
	}{
		{
			name:      "Empty roles",
			roles:     nil,
			checkRole: "role1",
			expected:  false,
		},
		{
			name: "Role exists",
			roles: []v1alpha1.Role{
				"role1",
				"role2",
			},
			checkRole: "role1",
			expected:  true,
		},
		{
			name: "Role does not exist",
			roles: []v1alpha1.Role{
				"role1",
				"role2",
			},
			checkRole: "role3",
			expected:  false,
		},
		{
			name: "Check duplicate role",
			roles: []v1alpha1.Role{
				"role1",
				"role1",
			},
			checkRole: "role1",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := v1alpha1.ClusterNode{
				Spec: v1alpha1.ClusterNodeSpec{
					Roles: tt.roles,
				},
			}
			result := HasRole(node, tt.checkRole)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// 辅助函数，用于检查切片中是否包含某个角色
func contains(roles []v1alpha1.Role, role v1alpha1.Role) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// TestRemoveRole tests the RemoveRole function.
func TestRemoveRole(t *testing.T) {
	tests := []struct {
		name       string
		initial    []v1alpha1.Role
		removeRole v1alpha1.Role
		expected   []v1alpha1.Role
	}{
		{
			name:       "Remove from empty roles",
			initial:    nil,
			removeRole: "role1",
			expected:   nil,
		},
		{
			name: "Remove existing role",
			initial: []v1alpha1.Role{
				"role1",
				"role2",
			},
			removeRole: "role1",
			expected:   []v1alpha1.Role{"role2"},
		},
		{
			name: "Remove non-existing role",
			initial: []v1alpha1.Role{
				"role1",
				"role2",
			},
			removeRole: "role3",
			// 不变
			expected: []v1alpha1.Role{
				"role1",
				"role2",
			},
		},
		{
			name: "Remove duplicate role",
			initial: []v1alpha1.Role{
				"role1",
				"role1",
				"role2",
			},
			removeRole: "role1",
			expected: []v1alpha1.Role{
				"role2",
			},
		},
		{
			name: "Remove last role",
			initial: []v1alpha1.Role{
				"role1",
			},
			removeRole: "role1",
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1alpha1.ClusterNode{
				Spec: v1alpha1.ClusterNodeSpec{
					Roles: tt.initial,
				},
			}
			RemoveRole(node, tt.removeRole)

			if len(node.Spec.Roles) != len(tt.expected) {
				t.Errorf("expected %d roles, got %d", len(tt.expected), len(node.Spec.Roles))
			}

			for _, expectedRole := range tt.expected {
				if !contains(node.Spec.Roles, expectedRole) {
					t.Errorf("expected role %s not found", expectedRole)
				}
			}
		})
	}
}
