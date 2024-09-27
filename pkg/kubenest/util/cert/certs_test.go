package cert

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
)

func TestCertConfig_defaultPublicKeyAlgorithm(t *testing.T) {
	// 测试场景 1：PublicKeyAlgorithm 未设置，应该设置为 x509.RSA
	config := &CertConfig{
		PublicKeyAlgorithm: x509.UnknownPublicKeyAlgorithm,
	}
	config.defaultPublicKeyAlgorithm()
	if config.PublicKeyAlgorithm != x509.RSA {
		t.Errorf("expected PublicKeyAlgorithm to be x509.RSA, got %v", config.PublicKeyAlgorithm)
	}

	// 测试场景 2：PublicKeyAlgorithm 已设置，不应更改
	config = &CertConfig{
		PublicKeyAlgorithm: x509.ECDSA,
	}
	config.defaultPublicKeyAlgorithm()
	if config.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("expected PublicKeyAlgorithm to remain x509.ECDSA, got %v", config.PublicKeyAlgorithm)
	}
}

func TestCertConfig_defaultNotAfter(t *testing.T) {
	// 测试场景 1：NotAfter 未设置，应该自动设置为当前时间加上常量值
	config := &CertConfig{
		NotAfter: nil,
	}
	config.defaultNotAfter()
	expectedNotAfter := time.Now().Add(constants.CertificateValidity)
	if config.NotAfter == nil || config.NotAfter.Sub(expectedNotAfter) > time.Second {
		t.Errorf("expected NotAfter to be %v, got %v", expectedNotAfter, config.NotAfter)
	}

	// 测试场景 2：NotAfter 已设置，不应更改
	expectedTime := time.Now().Add(24 * time.Hour)
	config = &CertConfig{
		NotAfter: &expectedTime,
	}
	config.defaultNotAfter()
	if config.NotAfter != &expectedTime {
		t.Errorf("expected NotAfter to remain %v, got %v", expectedTime, config.NotAfter)
	}
}

func TestGetDefaultCertList(t *testing.T) {
	certList := GetDefaultCertList()

	// 确认返回的 CertConfig 列表包含预期数量的配置
	expectedCertCount := 9
	if len(certList) != expectedCertCount {
		t.Fatalf("expected %d certs, but got %d", expectedCertCount, len(certList))
	}

	// 验证每个 CertConfig 的 Name 是否符合预期
	expectedNames := []string{
		constants.CaCertAndKeyName,               // CA cert
		constants.VirtualClusterCertAndKeyName,   // Admin cert
		constants.ApiserverCertAndKeyName,        // Apiserver cert
		constants.FrontProxyCaCertAndKeyName,     // Front proxy CA cert
		constants.FrontProxyClientCertAndKeyName, // Front proxy client cert
		constants.EtcdCaCertAndKeyName,           // ETCD CA cert
		constants.EtcdServerCertAndKeyName,       // ETCD server cert
		constants.EtcdClientCertAndKeyName,       // ETCD client cert
		constants.ProxyServerCertAndKeyName,      // Proxy server cert
	}

	for i, certConfig := range certList {
		if certConfig.Name != expectedNames[i] {
			t.Errorf("expected cert name %s, but got %s", expectedNames[i], certConfig.Name)
		}
	}
}

func TestVirtualClusterProxyServer(t *testing.T) {
	certConfig := VirtualClusterProxyServer()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.ProxyServerCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.ProxyServerCertAndKeyName, certConfig.Name)
	}
	if certConfig.CAName != constants.CaCertAndKeyName {
		t.Errorf("expected CAName to be %s, but got %s", constants.CaCertAndKeyName, certConfig.CAName)
	}
	if certConfig.Config.CommonName != "virtualCluster-proxy-server" {
		t.Errorf("expected CommonName to be virtualCluster-proxy-server, but got %s", certConfig.Config.CommonName)
	}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	if len(certConfig.Config.Usages) != len(expectedUsages) {
		t.Errorf("expected %d usages, but got %d", len(expectedUsages), len(certConfig.Config.Usages))
	}
	for i, usage := range certConfig.Config.Usages {
		if usage != expectedUsages[i] {
			t.Errorf("expected usage %v, but got %v", expectedUsages[i], usage)
		}
	}
}

func TestVirtualClusterCertEtcdCA(t *testing.T) {
	certConfig := VirtualClusterCertEtcdCA()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.EtcdCaCertAndKeyName, certConfig.Name)
	}
	if certConfig.Config.CommonName != "virtualcluster-etcd-ca" {
		t.Errorf("expected CommonName to be virtualcluster-etcd-ca, but got %s", certConfig.Config.CommonName)
	}
}

// Test VirtualClusterCertEtcdServer
func TestVirtualClusterCertEtcdServer(t *testing.T) {
	certConfig := VirtualClusterCertEtcdServer()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.EtcdServerCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.EtcdServerCertAndKeyName, certConfig.Name)
	}
	if certConfig.CAName != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, but got %s", constants.EtcdCaCertAndKeyName, certConfig.CAName)
	}
	if certConfig.Config.CommonName != "virtualCluster-etcd-server" {
		t.Errorf("expected CommonName to be virtualCluster-etcd-server, but got %s", certConfig.Config.CommonName)
	}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	if len(certConfig.Config.Usages) != len(expectedUsages) {
		t.Errorf("expected %d usages, but got %d", len(expectedUsages), len(certConfig.Config.Usages))
	}
	for i, usage := range certConfig.Config.Usages {
		if usage != expectedUsages[i] {
			t.Errorf("expected usage %v, but got %v", expectedUsages[i], usage)
		}
	}
}

// Test VirtualClusterCertEtcdClient
func TestVirtualClusterCertEtcdClient(t *testing.T) {
	certConfig := VirtualClusterCertEtcdClient()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.EtcdClientCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.EtcdClientCertAndKeyName, certConfig.Name)
	}
	if certConfig.CAName != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, but got %s", constants.EtcdCaCertAndKeyName, certConfig.CAName)
	}
	if certConfig.Config.CommonName != "virtualCluster-etcd-client" {
		t.Errorf("expected CommonName to be virtualCluster-etcd-client, but got %s", certConfig.Config.CommonName)
	}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	if len(certConfig.Config.Usages) != len(expectedUsages) {
		t.Errorf("expected %d usages, but got %d", len(expectedUsages), len(certConfig.Config.Usages))
	}
}

// Test VirtualClusterCertFrontProxyCA
func TestVirtualClusterCertFrontProxyCA(t *testing.T) {
	certConfig := VirtualClusterCertFrontProxyCA()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.FrontProxyCaCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.FrontProxyCaCertAndKeyName, certConfig.Name)
	}
	if certConfig.Config.CommonName != "front-proxy-ca" {
		t.Errorf("expected CommonName to be front-proxy-ca, but got %s", certConfig.Config.CommonName)
	}
}

// Test VirtualClusterFrontProxyClient
func TestVirtualClusterFrontProxyClient(t *testing.T) {
	certConfig := VirtualClusterFrontProxyClient()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.FrontProxyClientCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.FrontProxyClientCertAndKeyName, certConfig.Name)
	}
	if certConfig.CAName != constants.FrontProxyCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, but got %s", constants.FrontProxyCaCertAndKeyName, certConfig.CAName)
	}
	if certConfig.Config.CommonName != "front-proxy-client" {
		t.Errorf("expected CommonName to be front-proxy-client, but got %s", certConfig.Config.CommonName)
	}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if len(certConfig.Config.Usages) != len(expectedUsages) {
		t.Errorf("expected %d usages, but got %d", len(expectedUsages), len(certConfig.Config.Usages))
	}
	for i, usage := range certConfig.Config.Usages {
		if usage != expectedUsages[i] {
			t.Errorf("expected usage %v, but got %v", expectedUsages[i], usage)
		}
	}
}

// Test VirtualClusterCertApiserver
func TestVirtualClusterCertApiserver(t *testing.T) {
	certConfig := VirtualClusterCertApiserver()

	// 验证 certConfig 的各项配置
	if certConfig.Name != constants.ApiserverCertAndKeyName {
		t.Errorf("expected Name to be %s, but got %s", constants.ApiserverCertAndKeyName, certConfig.Name)
	}
	if certConfig.CAName != constants.CaCertAndKeyName {
		t.Errorf("expected CAName to be %s, but got %s", constants.CaCertAndKeyName, certConfig.CAName)
	}
	if certConfig.Config.CommonName != "virtualCluster-apiserver" {
		t.Errorf("expected CommonName to be virtualCluster-apiserver, but got %s", certConfig.Config.CommonName)
	}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	if len(certConfig.Config.Usages) != len(expectedUsages) {
		t.Errorf("expected %d usages, but got %d", len(expectedUsages), len(certConfig.Config.Usages))
	}
	for i, usage := range certConfig.Config.Usages {
		if usage != expectedUsages[i] {
			t.Errorf("expected usage %v, but got %v", expectedUsages[i], usage)
		}
	}
}

// Test etcdServerAltNamesMutator
//func TestEtcdServerAltNamesMutator(t *testing.T) {
//	cfg := &AltNamesMutatorConfig{
//		Name:      "test",
//		Namespace: "default",
//		ClusterIPs: []string{
//			"10.96.0.1",
//			"10.96.0.2",
//		},
//	}
//
//	altNames, err := etcdServerAltNamesMutator(cfg)
//	if err != nil {
//		t.Fatalf("unexpected error: %v", err)
//	}
//
//	// 验证 DNS 名称
//	expectedDNSNames := []string{
//		"localhost",
//		"test.default.svc.cluster.local",
//		"*.test.default.svc.cluster.local",
//	}
//	if len(altNames.DNSNames) != len(expectedDNSNames) {
//		t.Fatalf("expected %d DNS names, but got %d", len(expectedDNSNames), len(altNames.DNSNames))
//	}
//	for i, dns := range altNames.DNSNames {
//		if dns != expectedDNSNames[i] {
//			t.Errorf("expected DNS name %s, but got %s", expectedDNSNames[i], dns)
//		}
//	}
//
//	// 验证 IP 地址
//	expectedIPs := []net.IP{
//		net.ParseIP("::1"),
//		net.IPv4(127, 0, 0, 1),
//		net.ParseIP("10.96.0.1"),
//		net.ParseIP("10.96.0.2"),
//	}
//	if len(altNames.IPs) != len(expectedIPs) {
//		t.Fatalf("expected %d IPs, but got %d", len(expectedIPs), len(altNames.IPs))
//	}
//	for i, ip := range altNames.IPs {
//		if !ip.Equal(expectedIPs[i]) {
//			t.Errorf("expected IP %v, but got %v", expectedIPs[i], ip)
//		}
//	}
//}
