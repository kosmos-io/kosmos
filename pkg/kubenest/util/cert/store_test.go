package cert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func TestVirtualClusterCertStore_AddCert(t *testing.T) {
	store := NewCertStore()
	cert := &VirtualClusterCert{pairName: "test-cert", cert: []byte("test-cert-data"), key: []byte("test-key-data")}
	store.AddCert(cert)

	// 确保添加的证书可以被正确获取
	retrievedCert := store.GetCert("test-cert")
	assert.NotNil(t, retrievedCert, "Expected to retrieve the added certificate")
	assert.Equal(t, cert, retrievedCert, "Retrieved certificate should match the added certificate")
}

func TestVirtualClusterCertStore_GetCert(t *testing.T) {
	store := NewCertStore()
	cert := &VirtualClusterCert{pairName: "test-cert", cert: []byte("test-cert-data"), key: []byte("test-key-data")}
	store.AddCert(cert)

	// 测试获取存在的证书
	retrievedCert := store.GetCert("test-cert")
	assert.NotNil(t, retrievedCert, "Expected to retrieve the added certificate")
	assert.Equal(t, cert, retrievedCert, "Retrieved certificate should match the added certificate")

	// 测试获取不存在的证书
	retrievedCert = store.GetCert("nonexistent-cert")
	assert.Nil(t, retrievedCert, "Expected no certificate for a nonexistent name")
}

func TestVirtualClusterCertStore_CertList(t *testing.T) {
	store := NewCertStore()
	cert1 := &VirtualClusterCert{pairName: "cert1"}
	cert2 := &VirtualClusterCert{pairName: "cert2"}
	store.AddCert(cert1)
	store.AddCert(cert2)

	certs := store.CertList()
	assert.Len(t, certs, 2, "Expected the certificate list to contain 2 certificates")
	assert.Contains(t, certs, cert1, "Expected cert1 to be in the certificate list")
	assert.Contains(t, certs, cert2, "Expected cert2 to be in the certificate list")
}

func TestVirtualClusterCertStore_LoadCertFromSecret(t *testing.T) {
	store := NewCertStore()

	// 创建一个包含证书和密钥的 secret
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"test-cert" + constants.CertExtension: []byte("test-cert-data"),
			"test-cert" + constants.KeyExtension:  []byte("test-key-data"),
		},
	}

	// 加载证书
	err := store.LoadCertFromSecret(secret)
	assert.NoError(t, err, "Expected no error when loading cert from secret")

	// 确保可以成功获取证书
	cert := store.GetCert("test-cert")
	assert.NotNil(t, cert, "Expected to retrieve the certificate after loading from secret")
	assert.Equal(t, []byte("test-cert-data"), cert.cert, "Expected cert data to match")
	assert.Equal(t, []byte("test-key-data"), cert.key, "Expected key data to match")
}

func TestVirtualClusterCertStore_LoadCertFromEmptySecret(t *testing.T) {
	store := NewCertStore()

	// 创建一个空的 secret
	secret := &corev1.Secret{
		Data: map[string][]byte{},
	}

	// 尝试加载证书，应该返回错误
	err := store.LoadCertFromSecret(secret)
	assert.Error(t, err, "Expected error when loading cert from empty secret")
	assert.Equal(t, "cert data is empty", err.Error(), "Expected error message to match")
}
