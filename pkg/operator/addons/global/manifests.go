package global

const clusterlinkNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
`

type NamespaceReplace struct {
	Namespace string
}
