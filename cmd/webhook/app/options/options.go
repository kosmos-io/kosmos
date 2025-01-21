package options

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/webhook"
)

type Options struct {
	LeaderElection       componentbaseconfig.LeaderElectionConfiguration
	KubernetesOptions    KubernetesOptions
	WebhookServerOptions WebhookServerOptions
	PodValidatorOptions  webhook.PodValidatorOptions
}

type KubernetesOptions struct {
	KubeConfig string  `json:"kubeconfig" yaml:"kubeconfig"`
	Master     string  `json:"master,omitempty" yaml:"master,omitempty"`
	QPS        float32 `json:"qps,omitempty" yaml:"qps,omitempty"`
	Burst      int     `json:"burst,omitempty" yaml:"burst,omitempty"`
}

type WebhookServerOptions struct {
	Host     string
	Port     int
	CertDir  string
	CertName string
	KeyName  string
}

func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: utils.DefaultNamespace,
			ResourceName:      "network-manager",
		},
	}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceName, "leader-elect-resource-name", "kosmos-webhook", "The name of resource object that is used for locking during leader election.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", utils.DefaultNamespace, "The namespace of resource object that is used for locking during leader election.")
	flags.Float32Var(&o.KubernetesOptions.QPS, "kube-qps", 40.0, "QPS to use while talking with kube-apiserver.")
	flags.IntVar(&o.KubernetesOptions.Burst, "kube-burst", 60, "Burst to use while talking with kube-apiserver.")
	flags.StringVar(&o.KubernetesOptions.KubeConfig, "kubeconfig", "", "Path for kubernetes kubeconfig file, if left blank, will use in cluster way.")
	flags.StringVar(&o.KubernetesOptions.Master, "master", "", "Used to generate kubeconfig for downloading, if not specified, will use host in kubeconfig.")
	flags.StringVar(&o.WebhookServerOptions.Host, "bind-address", "0.0.0.0", "The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.WebhookServerOptions.Port, "secure-port", 9443, "The secure port on which to serve HTTPS.")
	flags.StringVar(&o.WebhookServerOptions.CertDir, "cert-dir", "/etc/certs", "The directory that contains the server key and certificate.")
	flags.StringVar(&o.WebhookServerOptions.CertName, "tls-cert-file-name", "tls.crt", "The name of server certificate.")
	flags.StringVar(&o.WebhookServerOptions.KeyName, "tls-private-key-file-name", "tls.key", "The name of server key.")
	flags.StringArrayVar(&o.PodValidatorOptions.UsernamesNeedToPrevent, "usernames-need-to-prevent", []string{"system:serviceaccount:kube-system:node-controller"}, "Usernames that need to prevent deleting pods on NotReady kosmos nodes.")
}
