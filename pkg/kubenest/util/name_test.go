package util

import (
	"testing"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func TestGetAPIServerName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-apiserver"
	if result := GetAPIServerName(name); result != expected {
		t.Errorf("GetAPIServerName() = %v, want %v", result, expected)
	}
}

func TestGetEtcdClientServerName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-etcd-client"
	if result := GetEtcdClientServerName(name); result != expected {
		t.Errorf("GetEtcdClientServerName() = %v, want %v", result, expected)
	}
}

func TestGetKonnectivityServerName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-konnectivity-server"
	if result := GetKonnectivityServerName(name); result != expected {
		t.Errorf("GetKonnectivityServerName() = %v, want %v", result, expected)
	}
}

func TestGetKonnectivityAPIServerName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-apiserver-konnectivity"
	if result := GetKonnectivityAPIServerName(name); result != expected {
		t.Errorf("GetKonnectivityAPIServerName() = %v, want %v", result, expected)
	}
}

func TestGetEtcdServerName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-etcd"
	if result := GetEtcdServerName(name); result != expected {
		t.Errorf("GetEtcdServerName() = %v, want %v", result, expected)
	}
}

func TestGetCertName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-cert"
	if result := GetCertName(name); result != expected {
		t.Errorf("GetCertName() = %v, want %v", result, expected)
	}
}

func TestGetEtcdCertName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-etcd-cert"
	if result := GetEtcdCertName(name); result != expected {
		t.Errorf("GetEtcdCertName() = %v, want %v", result, expected)
	}
}

func TestGetAdminConfigSecretName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-" + constants.AdminConfig
	if result := GetAdminConfigSecretName(name); result != expected {
		t.Errorf("GetAdminConfigSecretName() = %v, want %v", result, expected)
	}
}

func TestGetAdminConfigClusterIPSecretName(t *testing.T) {
	name := "test-cluster"
	expected := "test-cluster-admin-config-clusterip"
	if result := GetAdminConfigClusterIPSecretName(name); result != expected {
		t.Errorf("GetAdminConfigClusterIPSecretName() = %v, want %v", result, expected)
	}
}
