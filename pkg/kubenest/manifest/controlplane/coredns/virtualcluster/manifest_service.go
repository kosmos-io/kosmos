package virtualcluster

const (
	CoreDnsService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: CoreDNS
  name: kube-dns
  namespace: kube-system
spec:
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: {{ .DNSPort }}
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: {{ .DNSTCPPort }}
  - name: metrics
    port: 9153
    protocol: TCP
    targetPort: {{ .MetricsPort }}

`
)
