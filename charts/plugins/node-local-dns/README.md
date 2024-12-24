# Node-Local-DNS

Kosmos-kubenest plugin NodeLocalDNS helm chart

## Summary

The chart install NodeLocalDNS set according to <https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/>.

It is designed to work both with Iptables and IPVS setup.

Latest available `node-local-dns` image can be found at [node-local-dns google container repository](https://console.cloud.google.com/gcr/images/google-containers/GLOBAL/k8s-dns-node-cache)

## Values

| Key                       | Type | Default                                    | Description |
|---------------------------|------|--------------------------------------------|-------------|
| image.repository          | string | `"registry.k8s.io/dns/k8s-dns-node-cache"` |  |
| image.version             | string | `"1.23.1"`                                 |  |
| image.pullPolicy          | string | `"IfNotPresent"`                           |  |
| config.domain             | string | `"cluster.local"`                          |  |
| config.kubeDNS            | string | `"xxx.xxx.xxx.xxx"`                        |  |
| config.localDNS           | string | `"xxx.xxx.xxx.xxx"`                        |  |
| config.clusterDNS         | string | `"xxx.xxx.xxx.xxx"`                        |  |
| resources.requests.cpu    | string | `"25m"`                                    |  |
| resources.requests.memory | string | `"5Mi"`                                    |  |
| tolerations[0].key        | string | `"CriticalAddonsOnly"`                     |  |
| tolerations[0].operator   | string | `"Exists"`                                 |  |
| tolerations[1].effect     | string | `"NoExecute"`                              |  |
| tolerations[1].operator   | string | `"Exists"`                                 |  |
| tolerations[2].effect     | string | `"NoSchedule"`                             |  |
| tolerations[2].operator   | string | `"Exists"`                                 |  |
| nodeSelector              | object | `{}`                                       |  |
| affinity                  | object | `{}`                                       |  |
