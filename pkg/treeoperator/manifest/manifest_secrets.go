package manifest

const (
	ClusterTreeClusterManagerSecret = `---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Name }}-clustertree-cluster-manager
  namespace: {{ .Namespace }}
type: Opaque
data:
  cert.pem: {{ .Cert }}
  key.pem: {{ .Key }}
  kubeconfig: {{ .Kubeconfig }}
`
)

type SecretReplace struct {
	Namespace  string
	Cert       string
	Key        string
	Kubeconfig string
	Name       string
}
