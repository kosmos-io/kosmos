package util

import (
	"testing"
)

func TestGenerateDeployment(t *testing.T) {
	template := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test-container
        image: nginx
`

	obj := struct{}{} // 可以根据需要定义实际对象

	deployment, err := GenerateDeployment(template, obj)

	if err != nil {
		t.Errorf("expected no error")
	}

	if deployment == nil {
		t.Errorf("expected deployment is not nil")
		return
	}

	if deployment.Name != "test-deployment" {
		t.Errorf("expected deployment name is %v", "test-deployment")
	}
}

func TestGenerateDaemonSet(t *testing.T) {
	template := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test-daemonset
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test-container
        image: nginx
`

	obj := struct{}{} // 可以根据需要定义实际对象

	daemonSet, err := GenerateDaemonSet(template, obj)

	if err != nil {
		t.Errorf("expected no error")
	}

	if daemonSet == nil {
		t.Errorf("expected daemonSet is not nil")
		return
	}

	if daemonSet.Name != "test-daemonset" {
		t.Errorf("expected daemonSet name is %v", "test-daemonset")
	}
}

func TestGenerateServiceAccount(t *testing.T) {
	template := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-serviceaccount
`

	obj := struct{}{} // 可以根据需要定义实际对象

	serviceAccount, err := GenerateServiceAccount(template, obj)
	if err != nil {
		t.Errorf("expected no error")
	}

	if serviceAccount == nil {
		t.Errorf("expected serviceAccount is not nil")
		return
	}

	if serviceAccount.Name != "test-serviceaccount" {
		t.Errorf("expected serviceAccount name is %v", "test-serviceaccount")
	}
}

func TestGenerateClusterRole(t *testing.T) {
	template := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-clusterrole
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
`

	obj := struct{}{} // 可以根据需要定义实际对象

	clusterRole, err := GenerateClusterRole(template, obj)

	if err != nil {
		t.Errorf("expected no error")
	}

	if clusterRole == nil {
		t.Errorf("expected clusterRole is not nil")
		return
	}

	if clusterRole.Name != "test-clusterrole" {
		t.Errorf("expected clusterRole name is %v", "test-clusterrole")
	}
}

func TestGenerateClusterRoleBinding(t *testing.T) {
	template := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: test-clusterrolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: test-clusterrole
subjects:
- kind: ServiceAccount
  name: default
  namespace: default
`

	obj := struct{}{} // 可以根据需要定义实际对象

	clusterRoleBinding, err := GenerateClusterRoleBinding(template, obj)
	if err != nil {
		t.Errorf("expected no error")
	}

	if clusterRoleBinding == nil {
		t.Errorf("expected clusterrolebinding is not nil")
		return
	}

	if clusterRoleBinding.Name != "test-clusterrolebinding" {
		t.Errorf("expected clusterrolebinding name is %v", "test-clusterrolebinding")
	}
}
