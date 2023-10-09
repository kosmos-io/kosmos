package manifest

const (
	ClusterCR = `
apiVersion: kosmos.io/v1alpha1
kind: Cluster
metadata:
  name: {{ .ClusterName }}
spec:
  cni: {{ .CNI }}
  defaultNICName: {{ .DefaultNICName }}
  imageRepository: {{ .ImageRepository }} # Default: ghcr.io/kosmos-io
  networkType: {{ .NetworkType }} # Optional: gateway/p2p
`

	KnodeCR = `
apiVersion: kosmos.io/v1alpha1
kind: Knode
metadata:
  name: {{ .KnodeName }}
spec:
  nodeName: {{ .KnodeName }}
  type: k8s
  kubeconfig: {{ .KnodeKubeConfig }}
`
)

type ClusterReplace struct {
	ClusterName     string
	CNI             string
	DefaultNICName  string
	ImageRepository string
	NetworkType     string
}

type KnodeReplace struct {
	KnodeName       string
	KnodeKubeConfig string
}
