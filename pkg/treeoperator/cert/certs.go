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
)

const (
	// CertificateBlockType is a possible value for pem.Block.Type.
	CertificateBlockType           = "CERTIFICATE"
	rsaKeySize                     = 2048
	keyExtension                   = ".key"
	certExtension                  = ".crt"
	CertificateValidity            = time.Hour * 24 * 365
	CaCertAndKeyName               = "ca"
	VirtualClusterCertAndKeyName   = "virtualCluster"
	VirtualClusterSystemNamespace  = "virtualCluster-system"
	ApiserverCertAndKeyName        = "apiserver"
	EtcdCaCertAndKeyName           = "etcd-ca"
	EtcdServerCertAndKeyName       = "etcd-server"
	EtcdClientCertAndKeyName       = "etcd-client"
	FrontProxyCaCertAndKeyName     = "front-proxy-ca"
	FrontProxyClientCertAndKeyName = "front-proxy-client"
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
}

func (config *CertConfig) defaultPublicKeyAlgorithm() {
	if config.PublicKeyAlgorithm == x509.UnknownPublicKeyAlgorithm {
		config.PublicKeyAlgorithm = x509.RSA
	}
}

func (config *CertConfig) defaultNotAfter() {
	if config.NotAfter == nil {
		notAfter := time.Now().Add(CertificateValidity).UTC()
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
	}
}

func VirtualClusterCertEtcdCA() *CertConfig {
	return &CertConfig{
		Name: EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualcluster-etcd-ca",
		},
	}
}

func VirtualClusterCertEtcdServer() *CertConfig {
	return &CertConfig{
		Name:   EtcdServerCertAndKeyName,
		CAName: EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-etcd-server",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(etcdServerAltNamesMutator),
	}
}

func VirtualClusterCertEtcdClient() *CertConfig {
	return &CertConfig{
		Name:   EtcdClientCertAndKeyName,
		CAName: EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-etcd-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
	}
}

func VirtualClusterCertFrontProxyCA() *CertConfig {
	return &CertConfig{
		Name: FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-ca",
		},
	}
}

func VirtualClusterFrontProxyClient() *CertConfig {
	return &CertConfig{
		Name:   FrontProxyClientCertAndKeyName,
		CAName: FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
}

func etcdServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	etcdClientServiceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", fmt.Sprintf("%s-%s", cfg.Name, "etcd-client"), cfg.Namespace)
	etcdPeerServiceDNS := fmt.Sprintf("*.%s.%s.svc.cluster.local", fmt.Sprintf("%s-%s", cfg.Name, "etcd"), cfg.Namespace)

	altNames := &certutil.AltNames{
		DNSNames: []string{"localhost", etcdClientServiceDNS, etcdPeerServiceDNS},
		IPs:      []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	return altNames, nil
}

func VirtualClusterCertApiserver() *CertConfig {
	return &CertConfig{
		Name:   ApiserverCertAndKeyName,
		CAName: CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster-apiserver",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}

func VirtualClusterCertRootCA() *CertConfig {
	return &CertConfig{
		Name: CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "virtualCluster",
		},
	}
}

func VirtualClusterCertAdmin() *CertConfig {
	return &CertConfig{
		Name:   VirtualClusterCertAndKeyName,
		CAName: CaCertAndKeyName,
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

func apiServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	altNames := &certutil.AltNames{
		DNSNames: []string{
			"localhost",
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("*.%s.svc.cluster.local", VirtualClusterSystemNamespace),
			fmt.Sprintf("*.%s.svc", VirtualClusterSystemNamespace),
		},
		IPs: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv4(10, 237, 36, 119),
			net.IPv4(101, 126, 80, 149),
		},
	}

	if cfg.Namespace != VirtualClusterSystemNamespace {
		appendSANsToAltNames(altNames, []string{fmt.Sprintf("*.%s.svc.cluster.local", cfg.Namespace),
			fmt.Sprintf("*.%s.svc", cfg.Namespace)})
	}
	if len(cfg.ControlplaneAddr) > 0 {
		appendSANsToAltNames(altNames, []string{cfg.ControlplaneAddr})
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
	return pair + certExtension
}

// KeyName returns cert key file name. its default suffix is ".key".
func (cert *VirtualClusterCert) KeyName() string {
	pair := cert.pairName
	if len(pair) == 0 {
		pair = "cert"
	}
	return pair + keyExtension
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
		Type:  CertificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func GeneratePrivateKey(keyType x509.PublicKeyAlgorithm) (crypto.Signer, error) {
	if keyType == x509.ECDSA {
		return ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	}

	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
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
	notAfter := time.Now().Add(CertificateValidity).UTC()
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
		CAName: CaCertAndKeyName,
		Config: certutil.Config{
			CommonName:   "system:admin",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}
