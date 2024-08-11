package nodeserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	"github.com/kosmos.io/kosmos/pkg/cert"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server/api"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

func DefaultServerCiphers() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
}

type NodeServer struct {
	RootClient              client.Client
	GlobalLeafManager       leafUtils.LeafResourceManager
	GlobalLeafClientManager leafUtils.LeafClientResourceManager
}

type HttpConfig struct {
	listenAddr string
	handler    http.Handler
	tlsConfig  *tls.Config
}

func (s *NodeServer) getClient(ctx context.Context, namespace string, podName string) (kubernetes.Interface, *rest.Config, error) {
	nsname := types.NamespacedName{
		Namespace: namespace,
		Name:      podName,
	}

	rootPod := &corev1.Pod{}
	if err := s.RootClient.Get(ctx, nsname, rootPod); err != nil {
		return nil, nil, err
	}

	nodeName := rootPod.Spec.NodeName

	lr, err := s.GlobalLeafManager.GetLeafResourceByNodeName(nodeName)
	if err != nil {
		return nil, nil, err
	}

	actualClusterName := leafUtils.GetActualClusterName(lr.Cluster)
	lcr, err := s.GlobalLeafClientManager.GetLeafResource(actualClusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("get leaf client resource err: %v", err)
	}

	return lcr.Clientset, lcr.RestConfig, nil
}

func (s *NodeServer) RunHTTP(ctx context.Context, httpConfig HttpConfig) (func(), error) {
	if httpConfig.tlsConfig == nil {
		klog.Warning("TLS config not provided, not starting up http service")
		return func() {}, nil
	}
	if httpConfig.handler == nil {
		klog.Warning("No http handler, not starting up http service")
		return func() {}, nil
	}

	l, err := tls.Listen("tcp", httpConfig.listenAddr, httpConfig.tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error starting http listener")
	}

	klog.V(4).Info("Started TLS listener")

	srv := &http.Server{Handler: httpConfig.handler, TLSConfig: httpConfig.tlsConfig, ReadHeaderTimeout: 30 * time.Second}
	// nolint:errcheck
	go srv.Serve(l)
	klog.V(4).Infof("HTTP server running, port: %s", httpConfig.listenAddr)

	return func() {
		srv.Close()
		l.Close()
	}, nil
}

func (s *NodeServer) AttachRoutes(m *http.ServeMux) {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc(
		"/containerLogs/{namespace}/{pod}/{container}",
		api.ContainerLogsHandler(s.getClient),
	).Methods("GET")

	r.HandleFunc(
		"/exec/{namespace}/{pod}/{container}",
		api.ContainerExecHandler(
			api.ContainerExecOptions{
				StreamIdleTimeout:     30 * time.Second,
				StreamCreationTimeout: 30 * time.Second,
			},
			s.getClient,
		),
	).Methods("POST", "GET")

	// append func here
	// TODO: return node status, url: /stats/summary?only_cpu_and_memory=true

	r.NotFoundHandler = http.HandlerFunc(api.NotFound)

	m.Handle("/", r)
}

func loadKeyPair() (tls.Certificate, error) {
	CertPath := os.Getenv("APISERVER_CERT_LOCATION")
	KeyPath := os.Getenv("APISERVER_KEY_LOCATION")

	if CertPath == "" || KeyPath == "" {
		return tls.X509KeyPair(cert.GetCrt(), cert.GetKey())
	}
	return tls.LoadX509KeyPair(CertPath, KeyPath)
}

func (s *NodeServer) initTLSConfig() (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites:             DefaultServerCiphers(),
		ClientAuth:               tls.RequestClientCert,
	}

	cert, err := loadKeyPair()
	if err != nil {
		return nil, err
	}
	tlsCfg.Certificates = append(tlsCfg.Certificates, cert)

	CACertPath := os.Getenv("APISERVER_CA_CERT_LOCATION")
	if CACertPath != "" {
		pem, err := os.ReadFile(CACertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading ca cert pem: %w", err)
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert

		if tlsCfg.ClientCAs == nil {
			tlsCfg.ClientCAs = x509.NewCertPool()
		}
		if !tlsCfg.ClientCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("could not parse ca cert pem")
		}
	}

	return tlsCfg, nil
}

func (s *NodeServer) Start(ctx context.Context, opts *options.Options) error {
	tlsConfig, err := s.initTLSConfig()

	if err != nil {
		klog.Fatalf("Node http server start failed: %s", err)
		return err
	}

	handler := http.NewServeMux()
	s.AttachRoutes(handler)

	cancelHTTP, err := s.RunHTTP(ctx, HttpConfig{
		listenAddr: fmt.Sprintf(":%d", opts.ListenPort),
		tlsConfig:  tlsConfig,
		handler:    handler,
	})

	if err != nil {
		return err
	}
	defer cancelHTTP()

	<-ctx.Done()

	klog.V(4).Infof("Stop node http proxy")

	return nil
}
