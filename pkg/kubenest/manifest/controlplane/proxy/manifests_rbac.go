package proxy

const (
	ProxySA = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .SAName }}
  namespace: {{ .Namespace }}
`
)
