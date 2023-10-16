package manifest

const (
	CorednsService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    kosmos.io/app: coredns
  name: coredns
  namespace: {{ .Namespace }}
spec:
  ports:
    - name: dns
      port: 53
      protocol: UDP
      targetPort: 53
    - name: dns-tcp
      port: 53
      protocol: TCP
      targetPort: 53
    - name: metrics
      port: 9153
      protocol: TCP
      targetPort: 9153
  selector:
    kosmos.io/app: coredns
  sessionAffinity: None
  type: ClusterIP
`
)

type ServiceReplace struct {
	Namespace string
}
