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

	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/test/e2e/framework"
)

var (
	// pollInterval defines the interval time for a poll operation.
	pollInterval time.Duration
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout time.Duration

	kubeconfig = os.Getenv("KUBECONFIG")

	// host clusters
	hostContext           string
	hostKubeClient        kubernetes.Interface
	hostDynamicClient     dynamic.Interface
	hostClusterLinkClient versioned.Interface

	// e2e-leaf-node-cluster
	thirdContext       string
	thirdRestConfig    *rest.Config
	thirdKubeClient    kubernetes.Interface
	thirdDynamicClient dynamic.Interface
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
	flag.StringVar(&thirdContext, "third-context", "kind-cluster-member3", "name of the third member cluster context in kubeconfig file.")
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(bytes []byte) {
	// InitClient Initialize the client connecting to the HOST/FIRST/SECOND cluster
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())
	hostRestConfig, err := framework.LoadRESTClientConfig(kubeconfig, hostContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	hostKubeClient, err = kubernetes.NewForConfig(hostRestConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	hostDynamicClient, err = dynamic.NewForConfig(hostRestConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	hostClusterLinkClient, err = versioned.NewForConfig(hostRestConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())
	thirdRestConfig, err = framework.LoadRESTClientConfig(kubeconfig, thirdContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	thirdKubeClient, err = kubernetes.NewForConfig(thirdRestConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	thirdDynamicClient, err = dynamic.NewForConfig(thirdRestConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
})
