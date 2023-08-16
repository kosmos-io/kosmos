package e2e

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kosmos.io/clusterlink/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/clusterlink/test/e2e/framework"
)

var (
	// pollInterval defines the interval time for a poll operation.
	pollInterval time.Duration
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout time.Duration
)

var (
	kubeconfig        string
	hostContext       string
	restConfig        *rest.Config
	kubeClient        kubernetes.Interface
	dynamicClient     dynamic.Interface
	clusterLinkClient versioned.Interface
)

const (
	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 5
)

func init() {
	// usage ginkgo -- --poll-interval=5s --pollTimeout=5m
	// eg. ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/ -- --poll-interval=5s --pollTimeout=5m
	flag.DurationVar(&pollInterval, "poll-interval", 5*time.Second, "poll-interval defines the interval time for a poll operation")
	flag.DurationVar(&pollTimeout, "poll-timeout", 300*time.Second, "poll-timeout defines the time which the poll operation times out")
	flag.StringVar(&hostContext, "host-context", "kind-cluster-host", "name of the host cluster context in kubeconfig file.")
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(bytes []byte) {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())
	config, err := framework.LoadRESTClientConfig(kubeconfig, hostContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	restConfig = config
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	clusterLinkClient, err = versioned.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	dynamicClient, err = dynamic.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
})
