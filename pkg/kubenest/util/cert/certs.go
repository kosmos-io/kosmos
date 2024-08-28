package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

type CertConfig struct {
	Name                string
	CAName              string
	NotAfter            *time.Time
	PublicKeyAlgorithm  x509.PublicKeyAlgorithm
	Config              certutil.Config
	AltNamesMutatorFunc altNamesMutatorFunc
}

type altNamesMutatorFunc func(*AltNamesMutatorConfig, *CertConfig) error

type AltNamesMutatorConfig struct {
	Name             string
	Namespace        string
	ControlplaneAddr string
	ClusterIps       []string
	ExternalIP       string
	ExternalIPs      []string
	VipMap           map[string]string
}

func (config *CertConfig) defaultPublicKeyAlgorithm() {
	if config.PublicKeyAlgorithm == x509.UnknownPublicKeyAlgorithm {
		config.PublicKeyAlgorithm = x509.RSA
	}
}

func (config *CertConfig) defaultNotAfter() {
	if config.NotAfter == nil {
		notAfter := time.Now().Add(constants.CertificateValidity).UTC()
		config.NotAfter = &notAfter
	}
}

func GetDefaultCertList() []*CertConfig {
	return []*CertConfig{
		// virtual cluster cert config.
		VirtualClusterCertRootCA(),
		VirtualClusterCertAdmin(),
		VirtualClusterCertApiserver(),
		// front proxy cert config.
		VirtualClusterCertFrontProxyCA(),
		VirtualClusterFrontProxyClient(),
		// ETCD cert config.
		VirtualClusterCertEtcdCA(),
		VirtualClusterCertEtcdServer(),
		VirtualClusterCertEtcdClient(),
		// proxy server cert config.
		VirtualClusterProxyServer(),
	}
}

func VirtualClusterProxyServer() *CertConfig {
	return &CertConfig{
		Name:   constants.ProxyServerCertAndKeyName,
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-proxy-server",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(proxyServerAltNamesMutator),
	}
}

func VirtualClusterCertEtcdCA() *CertConfig {
	return &CertConfig{
		Name: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualcluster-etcd-ca",
		},
	}
}

func VirtualClusterCertEtcdServer() *CertConfig {
	return &CertConfig{
		Name:   constants.EtcdServerCertAndKeyName,
		CAName: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-etcd-server",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(etcdServerAltNamesMutator),
	}
}

func VirtualClusterCertEtcdClient() *CertConfig {
	return &CertConfig{
		Name:   constants.EtcdClientCertAndKeyName,
		CAName: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-etcd-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
	}
}

func VirtualClusterCertFrontProxyCA() *CertConfig {
	return &CertConfig{
		Name: constants.FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-ca",
		},
	}
}

func VirtualClusterFrontProxyClient() *CertConfig {
	return &CertConfig{
		Name:   constants.FrontProxyClientCertAndKeyName,
		CAName: constants.FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
}

func etcdServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	etcdClientServiceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", util.GetEtcdClientServerName(cfg.Name), cfg.Namespace)
	etcdPeerServiceDNS := fmt.Sprintf("*.%s.%s.svc.cluster.local", util.GetEtcdServerName(cfg.Name), cfg.Namespace)

	altNames := &certutil.AltNames{
		DNSNames: []string{"localhost", etcdClientServiceDNS, etcdPeerServiceDNS},
		IPs:      []net.IP{net.ParseIP("::1"), net.IPv4(127, 0, 0, 1)},
	}

	if len(cfg.ClusterIps) > 0 {
		for _, clusterIp := range cfg.ClusterIps {
			appendSANsToAltNames(altNames, []string{clusterIp})
		}
	}
	return altNames, nil
}

func VirtualClusterCertApiserver() *CertConfig {
	return &CertConfig{
		Name:   constants.ApiserverCertAndKeyName,
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-apiserver",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}

func VirtualClusterCertRootCA() *CertConfig {
	return &CertConfig{
		Name: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster",
		},
	}
}

func VirtualClusterCertAdmin() *CertConfig {
	return &CertConfig{
		Name:   constants.VirtualClusterCertAndKeyName,
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName:   "system:admin",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}

func makeAltNamesMutator(f func(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error)) altNamesMutatorFunc {
	return func(cfg *AltNamesMutatorConfig, cc *CertConfig) error {
		altNames, err := f(cfg)
		if err != nil {
			return err
		}

		cc.Config.AltNames = *altNames
		return nil
	}
}

func proxyServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	firstIPs, err := util.GetFirstIP(constants.ApiServerServiceSubnet)
	if err != nil {
		return nil, err
	}

	altNames := &certutil.AltNames{
		DNSNames: []string{
			"localhost",
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
		},
		IPs: append([]net.IP{
			net.ParseIP("::1"),
			net.IPv4(127, 0, 0, 1),
		}, firstIPs...),
	}

	if cfg.Namespace != constants.VirtualClusterSystemNamespace {
		appendSANsToAltNames(altNames, []string{fmt.Sprintf("*.%s.svc.cluster.local", cfg.Namespace),
			fmt.Sprintf("*.%s.svc", cfg.Namespace)})
	}
	if len(cfg.ControlplaneAddr) > 0 {
		appendSANsToAltNames(altNames, []string{cfg.ControlplaneAddr})
	}
	if len(cfg.ExternalIP) > 0 {
		appendSANsToAltNames(altNames, []string{cfg.ExternalIP})
	}

	if len(cfg.ExternalIPs) > 0 {
		for _, externalIp := range cfg.ExternalIPs {
			appendSANsToAltNames(altNames, []string{externalIp})
		}
	}

	if len(cfg.ClusterIps) > 0 {
		for _, clusterIp := range cfg.ClusterIps {
			appendSANsToAltNames(altNames, []string{clusterIp})
		}
	}
	return altNames, nil
}

func apiServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	firstIPs, err := util.GetFirstIP(constants.ApiServerServiceSubnet)
	if err != nil {
		return nil, err
	}

	altNames := &certutil.AltNames{
		DNSNames: []string{
			"localhost",
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"konnectivity-server.kube-system.svc.cluster.local",
			// fmt.Sprintf("*.%s.svc.cluster.local", constants.VirtualClusterSystemNamespace),
			fmt.Sprintf("*.%s.svc", constants.VirtualClusterSystemNamespace),
		},
		//TODO （考虑节点属于当前集群节点和非当前集群节点情况）
		IPs: append([]net.IP{
			net.ParseIP("::1"),
			net.IPv4(127, 0, 0, 1),
		}, firstIPs...),
	}

	if cfg.Namespace != constants.VirtualClusterSystemNamespace {
		appendSANsToAltNames(altNames, []string{fmt.Sprintf("*.%s.svc.cluster.local", cfg.Namespace),
			fmt.Sprintf("*.%s.svc", cfg.Namespace)})
	}
	if len(cfg.ControlplaneAddr) > 0 {
		appendSANsToAltNames(altNames, []string{cfg.ControlplaneAddr})
	}
	if len(cfg.ExternalIP) > 0 {
		appendSANsToAltNames(altNames, []string{cfg.ExternalIP})
	}

	if len(cfg.ExternalIPs) > 0 {
		for _, externalIp := range cfg.ExternalIPs {
			appendSANsToAltNames(altNames, []string{externalIp})
		}
	}

	if len(cfg.VipMap) > 0 {
		for _, vip := range cfg.VipMap {
			appendSANsToAltNames(altNames, []string{vip})
		}
	}
	if len(cfg.ClusterIps) > 0 {
		for _, clusterIp := range cfg.ClusterIps {
			appendSANsToAltNames(altNames, []string{clusterIp})
		}
	}
	return altNames, nil
}

func appendSANsToAltNames(altNames *certutil.AltNames, SANs []string) {
	for _, altname := range SANs {
		if ip := netutils.ParseIPSloppy(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(validation.IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else if len(validation.IsWildcardDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		}
	}
}

type VirtualClusterCert struct {
	pairName string
	caName   string
	cert     []byte
	key      []byte
}

// CertData returns certificate cert data.
func (cert *VirtualClusterCert) CertData() []byte {
	return cert.cert
}

// KeyData returns certificate key data.
func (cert *VirtualClusterCert) KeyData() []byte {
	return cert.key
}

// CertName returns cert file name. its default suffix is ".crt".
func (cert *VirtualClusterCert) CertName() string {
	pair := cert.pairName
	if len(pair) == 0 {
		pair = "cert"
	}
	return pair + constants.CertExtension
}

// KeyName returns cert key file name. its default suffix is ".key".
func (cert *VirtualClusterCert) KeyName() string {
	pair := cert.pairName
	if len(pair) == 0 {
		pair = "cert"
	}
	return pair + constants.KeyExtension
}

func NewCertificateAuthority(cc *CertConfig) (*VirtualClusterCert, error) {
	cc.defaultPublicKeyAlgorithm()

	key, err := GeneratePrivateKey(cc.PublicKeyAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("unable to create private key while generating CA certificate, err: %w", err)
	}

	cert, err := certutil.NewSelfSignedCACert(cc.Config, key)
	if err != nil {
		return nil, fmt.Errorf("unable to create self-signed CA certificate, err: %w", err)
	}

	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal private key to PEM, err: %w", err)
	}

	return &VirtualClusterCert{
		pairName: cc.Name,
		caName:   cc.CAName,
		cert:     EncodeCertPEM(cert),
		key:      encoded,
	}, nil
}

func CreateCertAndKeyFilesWithCA(cc *CertConfig, caCertData, caKeyData []byte) (*VirtualClusterCert, error) {
	if len(cc.Config.Usages) == 0 {
		return nil, fmt.Errorf("must specify at least one ExtKeyUsage")
	}

	cc.defaultNotAfter()
	cc.defaultPublicKeyAlgorithm()

	key, err := GeneratePrivateKey(cc.PublicKeyAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("unable to create private key, err: %w", err)
	}

	caCerts, err := certutil.ParseCertsPEM(caCertData)
	if err != nil {
		return nil, err
	}

	caKey, err := ParsePrivateKeyPEM(caKeyData)
	if err != nil {
		return nil, err
	}

	// Safely pick the first one because the sender's certificate must come first in the list.
	// For details, see: https://www.rfc-editor.org/rfc/rfc4346#section-7.4.2
	caCert := caCerts[0]

	cert, err := NewSignedCert(cc, key, caCert, caKey, false)
	if err != nil {
		return nil, err
	}

	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal private key to PEM, err: %w", err)
	}

	return &VirtualClusterCert{
		pairName: cc.Name,
		caName:   cc.CAName,
		cert:     EncodeCertPEM(cert),
		key:      encoded,
	}, nil
}

func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  constants.CertificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func GeneratePrivateKey(keyType x509.PublicKeyAlgorithm) (crypto.Signer, error) {
	if keyType == x509.ECDSA {
		return ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	}

	return rsa.GenerateKey(cryptorand.Reader, constants.RsaKeySize)
}

func ParsePrivateKeyPEM(keyData []byte) (crypto.Signer, error) {
	caPrivateKey, err := keyutil.ParsePrivateKeyPEM(keyData)
	if err != nil {
		return nil, err
	}

	// Allow RSA and ECDSA formats only
	var key crypto.Signer
	switch k := caPrivateKey.(type) {
	case *rsa.PrivateKey:
		key = k
	case *ecdsa.PrivateKey:
		key = k
	default:
		return nil, errors.New("the private key is neither in RSA nor ECDSA format")
	}

	return key, nil
}

func NewSignedCert(cc *CertConfig, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, isCA bool) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cc.Config.CommonName) == 0 {
		return nil, fmt.Errorf("must specify a CommonName")
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	RemoveDuplicateAltNames(&cc.Config.AltNames)
	notAfter := time.Now().Add(constants.CertificateValidity).UTC()
	if cc.NotAfter != nil {
		notAfter = *cc.NotAfter
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cc.Config.CommonName,
			Organization: cc.Config.Organization,
		},
		DNSNames:              cc.Config.AltNames.DNSNames,
		IPAddresses:           cc.Config.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             caCert.NotBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           cc.Config.Usages,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func RemoveDuplicateAltNames(altNames *certutil.AltNames) {
	if altNames == nil {
		return
	}

	if altNames.DNSNames != nil {
		altNames.DNSNames = sets.NewString(altNames.DNSNames...).List()
	}

	ipsKeys := make(map[string]struct{})
	var ips []net.IP
	for _, one := range altNames.IPs {
		if _, ok := ipsKeys[one.String()]; !ok {
			ipsKeys[one.String()] = struct{}{}
			ips = append(ips, one)
		}
	}
	altNames.IPs = ips
}

func VirtualClusterCertClient() *CertConfig {
	return &CertConfig{
		Name:   "virtualCluster-client",
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName:   "system:admin",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}
