package manifest

const (
	CorednsCorefile = `
apiVersion: v1
data:
  Corefile: |
    .:53 {
        errors
        health {
            lameduck 5s
        }
        ready
        kubernetes kosmos.local cluster.local in-addr.arpa ip6.arpa {
            pods insecure
            ttl 30
        }
        hosts /etc/add-hosts/customer-hosts . {
            fallthrough kosmos.local cluster.local in-addr.arpa ip6.arpa
        }
        prometheus :9153
        cache 30
        reload
        loadbalance
    }
kind: ConfigMap
metadata:
  name: coredns
  namespace: {{ .Namespace }}
`

	CorednsCustomerHosts = `
apiVersion: v1
data:
  customer-hosts: |
    #customer-hosts
    #10.10.10.10 myhost1
kind: ConfigMap
metadata:
  name: coredns-customer-hosts
  namespace: {{ .Namespace }}
`
)

type ConfigmapReplace struct {
	Namespace string
}
