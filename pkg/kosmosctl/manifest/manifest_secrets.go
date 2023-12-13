package manifest

const (
	ClusterTreeClusterManagerSecret = `---
apiVersion: v1
kind: Secret
metadata:
  name: clustertree-cluster-manager
  namespace: {{ .Namespace }}
type: Opaque
data:
  cert.pem: {{ .Cert }}
  key.pem: {{ .Key }}
`
)

type SecretReplace struct {
	Namespace string
	Cert      string
	Key       string
}
