package util

import (
	"reflect"
	"testing"
)

func TestCreateBasic(t *testing.T) {
	serverURL := "https://test-server"
	clusterName := "test-cluster"
	userName := "test-user"
	caCert := []byte("test-ca-cert")

	config := CreateBasic(serverURL, clusterName, userName, caCert)

	// Check if the returned config is as expected
	if config.CurrentContext != "test-user@test-cluster" {
		t.Errorf("CreateBasic() CurrentContext = %v, want %v", config.CurrentContext, "test-user@test-cluster")
	}

	if cluster, ok := config.Clusters[clusterName]; !ok {
		t.Errorf("CreateBasic() missing cluster %v", clusterName)
	} else {
		if cluster.Server != serverURL {
			t.Errorf("CreateBasic() cluster.Server = %v, want %v", cluster.Server, serverURL)
		}
		if !reflect.DeepEqual(cluster.CertificateAuthorityData, caCert) {
			t.Errorf("CreateBasic() cluster.CertificateAuthorityData = %v, want %v", cluster.CertificateAuthorityData, caCert)
		}
	}

	if ctx, ok := config.Contexts["test-user@test-cluster"]; !ok {
		t.Errorf("CreateBasic() missing context %v", "test-user@test-cluster")
	} else {
		if ctx.Cluster != clusterName {
			t.Errorf("CreateBasic() ctx.Cluster = %v, want %v", ctx.Cluster, clusterName)
		}
		if ctx.AuthInfo != userName {
			t.Errorf("CreateBasic() ctx.AuthInfo = %v, want %v", ctx.AuthInfo, userName)
		}
	}
}

func TestCreateWithCerts(t *testing.T) {
	serverURL := "https://test-server"
	clusterName := "test-cluster"
	userName := "test-user"
	caCert := []byte("test-ca-cert")
	clientKey := []byte("test-client-key")
	clientCert := []byte("test-client-cert")

	config := CreateWithCerts(serverURL, clusterName, userName, caCert, clientKey, clientCert)

	// Validate the basic config part
	if config.CurrentContext != "test-user@test-cluster" {
		t.Errorf("CreateWithCerts() CurrentContext = %v, want %v", config.CurrentContext, "test-user@test-cluster")
	}

	// Validate AuthInfo part
	if authInfo, ok := config.AuthInfos[userName]; !ok {
		t.Errorf("CreateWithCerts() missing AuthInfo for %v", userName)
	} else {
		if !reflect.DeepEqual(authInfo.ClientKeyData, clientKey) {
			t.Errorf("CreateWithCerts() authInfo.ClientKeyData = %v, want %v", authInfo.ClientKeyData, clientKey)
		}
		if !reflect.DeepEqual(authInfo.ClientCertificateData, clientCert) {
			t.Errorf("CreateWithCerts() authInfo.ClientCertificateData = %v, want %v", authInfo.ClientCertificateData, clientCert)
		}
	}
}
