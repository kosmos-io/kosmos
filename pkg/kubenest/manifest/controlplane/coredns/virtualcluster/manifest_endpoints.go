package virtualcluster

const (
	CoreDnsEndpoints = `
apiVersion: v1
kind: Endpoints
metadata:
  name: kube-dns
  namespace: kube-system
subsets:
- addresses:
  - ip: {{ .HostNodeAddress }}
  ports:
  - name: dns
    port: {{ .DNSPort }}
    protocol: UDP
  - name: metrics
    port: {{ .MetricsPort }}
    protocol: TCP
`
)
