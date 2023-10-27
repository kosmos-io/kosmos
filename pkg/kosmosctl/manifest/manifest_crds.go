package manifest

const (
	ClusterlinkClusterNode = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusternodes.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: ClusterNode
    listKind: ClusterNodeList
    plural: clusternodes
    singular: clusternode
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - jsonPath: .spec.roles
      name: ROLES
      type: string
    - jsonPath: .spec.interfaceName
      name: INTERFACE
      type: string
    - jsonPath: .spec.ip
      name: IP
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            properties:
              clusterName:
                type: string
              interfaceName:
                type: string
              ip:
                type: string
              ip6:
                type: string
              nodeName:
                type: string
              podCIDRs:
                items:
                  type: string
                type: array
              roles:
                items:
                  type: string
                type: array
            type: object
          status:
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`

	ClusterlinkCluster = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusters.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: Cluster
    listKind: ClusterList
    plural: clusters
    singular: cluster
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - jsonPath: .spec.networkType
      name: NETWORK_TYPE
      type: string
    - jsonPath: .spec.ipFamily
      name: IP_FAMILY
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: Spec is the specification for the behaviour of the cluster.
            properties:
              bridgeCIDRs:
                default:
                  ip: 220.0.0.0/8
                  ip6: 9470::/16
                properties:
                  ip:
                    type: string
                  ip6:
                    type: string
                required:
                - ip
                - ip6
                type: object
              cni:
                default: calico
                type: string
              defaultNICName:
                default: '*'
                type: string
              globalCIDRsMap:
                additionalProperties:
                  type: string
                type: object
              imageRepository:
                type: string
              ipFamily:
                default: all
                type: string
              kubeconfig:
                format: byte
                type: string
              localCIDRs:
                default:
                  ip: 210.0.0.0/8
                  ip6: 9480::/16
                properties:
                  ip:
                    type: string
                  ip6:
                    type: string
                required:
                - ip
                - ip6
                type: object
              namespace:
                default: {{ .Namespace }}
                type: string
              networkType:
                default: p2p
                enum:
                - p2p
                - gateway
                type: string
              nicNodeNames:
                items:
                  properties:
                    interfaceName:
                      type: string
                    nodeName:
                      items:
                        type: string
                      type: array
                  required:
                  - interfaceName
                  - nodeName
                  type: object
                type: array
              useIPPool:
                default: false
                type: boolean
            type: object
          status:
            description: Status describes the current status of a cluster.
            properties:
              podCIDRs:
                items:
                  type: string
                type: array
              serviceCIDRs:
                items:
                  type: string
                type: array
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources: {}
`

	ClusterlinkNodeConfig = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: nodeconfigs.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: NodeConfig
    listKind: NodeConfigList
    plural: nodeconfigs
    singular: nodeconfig
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            properties:
              arps:
                items:
                  properties:
                    dev:
                      type: string
                    ip:
                      type: string
                    mac:
                      type: string
                  required:
                  - dev
                  - ip
                  - mac
                  type: object
                type: array
              devices:
                items:
                  properties:
                    addr:
                      type: string
                    bindDev:
                      type: string
                    id:
                      format: int32
                      type: integer
                    mac:
                      type: string
                    name:
                      type: string
                    port:
                      format: int32
                      type: integer
                    type:
                      type: string
                  required:
                  - addr
                  - bindDev
                  - id
                  - mac
                  - name
                  - port
                  - type
                  type: object
                type: array
              fdbs:
                items:
                  properties:
                    dev:
                      type: string
                    ip:
                      type: string
                    mac:
                      type: string
                  required:
                  - dev
                  - ip
                  - mac
                  type: object
                type: array
              iptables:
                items:
                  properties:
                    chain:
                      type: string
                    rule:
                      type: string
                    table:
                      type: string
                  required:
                  - chain
                  - rule
                  - table
                  type: object
                type: array
              routes:
                items:
                  properties:
                    cidr:
                      type: string
                    dev:
                      type: string
                    gw:
                      type: string
                  required:
                  - cidr
                  - dev
                  - gw
                  type: object
                type: array
            type: object
          status:
            properties:
              lastChangeTime:
                format: date-time
                type: string
              lastSyncTime:
                format: date-time
                type: string
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`

	ClusterTreeKnode = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: knodes.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: Knode
    listKind: KnodeList
    plural: knodes
    singular: knode
  scope: Cluster
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            properties:
              disableTaint:
                type: boolean
              kubeconfig:
                format: byte
                type: string
              nodeName:
                type: string
              type:
                default: k8s
                type: string
            type: object
          status:
            properties:
              apiserver:
                type: string
              conditions:
                items:
                  description: "Condition contains details for one aspect of the current
                    state of this API Resource. --- This struct is intended for direct
                    use as an array at the field path .status.conditions."
                  properties:
                    lastTransitionTime:
                      description: lastTransitionTime is the last time the condition
                        transitioned from one status to another. This should be when
                        the underlying condition changed.  If that is not known, then
                        using the time when the API field changed is acceptable.
                      format: date-time
                      type: string
                    message:
                      description: message is a human readable message indicating
                        details about the transition. This may be an empty string.
                      maxLength: 32768
                      type: string
                    observedGeneration:
                      description: observedGeneration represents the .metadata.generation
                        that the condition was set based upon. For instance, if .metadata.generation
                        is currently 12, but the .status.conditions[x].observedGeneration
                        is 9, the condition is out of date with respect to the current
                        state of the instance.
                      format: int64
                      minimum: 0
                      type: integer
                    reason:
                      description: reason contains a programmatic identifier indicating
                        the reason for the condition's last transition. Producers
                        of specific condition types may define expected values and
                        meanings for this field, and whether the values are considered
                        a guaranteed API. The value should be a CamelCase string.
                        This field may not be empty.
                      maxLength: 1024
                      minLength: 1
                      pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                      type: string
                    status:
                      description: status of the condition, one of True, False, Unknown.
                      enum:
                      - "True"
                      - "False"
                      - Unknown
                      type: string
                    type:
                      description: type of condition in CamelCase or in foo.example.com/CamelCase.
                        --- Many .condition.type values are consistent across resources
                        like Available, but because arbitrary conditions can be useful
                        (see .node.status.conditions), the ability to deconflict is
                        important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
                      maxLength: 316
                      pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                      type: string
                  required:
                  - lastTransitionTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              version:
                type: string
            type: object
        type: object
    served: true
    storage: true
`
	ServiceImport = `# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: serviceimports.multicluster.x-k8s.io
spec:
  group: multicluster.x-k8s.io
  scope: Namespaced
  names:
    plural: serviceimports
    singular: serviceimport
    kind: ServiceImport
    shortNames:
    - svcim
  versions:
  - name: v1alpha1
    served: true
    storage: true
    subresources:
      status: {}
    additionalPrinterColumns:
    - name: Type
      type: string
      description: The type of this ServiceImport
      jsonPath: .spec.type
    - name: IP
      type: string
      description: The VIP for this ServiceImport
      jsonPath: .spec.ips
    - name: Age
      type: date
      jsonPath: .metadata.creationTimestamp
    "schema":
      "openAPIV3Schema":
        description: ServiceImport describes a service imported from clusters in a
          ClusterSet.
        type: object
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: spec defines the behavior of a ServiceImport.
            type: object
            required:
            - ports
            - type
            properties:
              ips:
                description: ip will be used as the VIP for this service when type
                  is ClusterSetIP.
                type: array
                maxItems: 1
                items:
                  type: string
              ports:
                type: array
                items:
                  description: ServicePort represents the port on which the service
                    is exposed
                  type: object
                  required:
                  - port
                  properties:
                    appProtocol:
                      description: The application protocol for this port. This field
                        follows standard Kubernetes label syntax. Un-prefixed names
                        are reserved for IANA standard service names (as per RFC-6335
                        and http://www.iana.org/assignments/service-names). Non-standard
                        protocols should use prefixed names such as mycompany.com/my-custom-protocol.
                        Field can be enabled with ServiceAppProtocol feature gate.
                      type: string
                    name:
                      description: The name of this port within the service. This
                        must be a DNS_LABEL. All ports within a ServiceSpec must have
                        unique names. When considering the endpoints for a Service,
                        this must match the 'name' field in the EndpointPort. Optional
                        if only one ServicePort is defined on this service.
                      type: string
                    port:
                      description: The port that will be exposed by this service.
                      type: integer
                      format: int32
                    protocol:
                      description: The IP protocol for this port. Supports "TCP",
                        "UDP", and "SCTP". Default is TCP.
                      type: string
                x-kubernetes-list-type: atomic
              sessionAffinity:
                description: 'Supports "ClientIP" and "None". Used to maintain session
                  affinity. Enable client IP based session affinity. Must be ClientIP
                  or None. Defaults to None. Ignored when type is Headless More info:
                  https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies'
                type: string
              sessionAffinityConfig:
                description: sessionAffinityConfig contains session affinity configuration.
                type: object
                properties:
                  clientIP:
                    description: clientIP contains the configurations of Client IP
                      based session affinity.
                    type: object
                    properties:
                      timeoutSeconds:
                        description: timeoutSeconds specifies the seconds of ClientIP
                          type session sticky time. The value must be >0 && <=86400(for
                          1 day) if ServiceAffinity == "ClientIP". Default value is
                          10800(for 3 hours).
                        type: integer
                        format: int32
              type:
                description: type defines the type of this service. Must be ClusterSetIP
                  or Headless.
                type: string
                enum:
                - ClusterSetIP
                - Headless
          status:
            description: status contains information about the exported services that
              form the multi-cluster service referenced by this ServiceImport.
            type: object
            properties:
              clusters:
                description: clusters is the list of exporting clusters from which
                  this service was derived.
                type: array
                items:
                  description: ClusterStatus contains service configuration mapped
                    to a specific source cluster
                  type: object
                  required:
                  - cluster
                  properties:
                    cluster:
                      description: cluster is the name of the exporting cluster. Must
                        be a valid RFC-1123 DNS label.
                      type: string
                x-kubernetes-list-map-keys:
                - cluster
                x-kubernetes-list-type: map
`
	ServiceExport = `# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: serviceexports.multicluster.x-k8s.io
spec:
  group: multicluster.x-k8s.io
  scope: Namespaced
  names:
    plural: serviceexports
    singular: serviceexport
    kind: ServiceExport
    shortNames:
    - svcex
  versions:
  - name: v1alpha1
    served: true
    storage: true
    subresources:
      status: {}
    additionalPrinterColumns:
    - name: Age
      type: date
      jsonPath: .metadata.creationTimestamp
    "schema":
      "openAPIV3Schema":
        description: ServiceExport declares that the Service with the same name and
          namespace as this export should be consumable from other clusters.
        type: object
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          status:
            description: status describes the current state of an exported service.
              Service configuration comes from the Service that had the same name
              and namespace as this ServiceExport. Populated by the multi-cluster
              service implementation's controller.
            type: object
            properties:
              conditions:
                type: array
                items:
                  description: "Condition contains details for one aspect of the current
                    state of this API Resource. --- This struct is intended for direct
                    use as an array at the field path .status.conditions.  For example,
                    type FooStatus struct{     // Represents the observations of a
                    foo's current state.     // Known .status.conditions.type are:
                    \"Available\", \"Progressing\", and \"Degraded\"     // +patchMergeKey=type
                    \    // +patchStrategy=merge     // +listType=map     // +listMapKey=type
                    \    Conditions []metav1.Condition //json:\"conditions,omitempty\"
                    patchStrategy:\"merge\" patchMergeKey:\"type\" protobuf:\"bytes,1,rep,name=conditions\"
                    \n     // other fields }"
                  type: object
                  required:
                  - lastTransitionTime
                  - message
                  - reason
                  - status
                  - type
                  properties:
                    lastTransitionTime:
                      description: lastTransitionTime is the last time the condition
                        transitioned from one status to another. This should be when
                        the underlying condition changed.  If that is not known, then
                        using the time when the API field changed is acceptable.
                      type: string
                      format: date-time
                    message:
                      description: message is a human readable message indicating
                        details about the transition. This may be an empty string.
                      type: string
                      maxLength: 32768
                    observedGeneration:
                      description: observedGeneration represents the .metadata.generation
                        that the condition was set based upon. For instance, if .metadata.generation
                        is currently 12, but the .status.conditions[x].observedGeneration
                        is 9, the condition is out of date with respect to the current
                        state of the instance.
                      type: integer
                      format: int64
                      minimum: 0
                    reason:
                      description: reason contains a programmatic identifier indicating
                        the reason for the condition's last transition. Producers
                        of specific condition types may define expected values and
                        meanings for this field, and whether the values are considered
                        a guaranteed API. The value should be a CamelCase string.
                        This field may not be empty.
                      type: string
                      maxLength: 1024
                      minLength: 1
                      pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                    status:
                      description: status of the condition, one of True, False, Unknown.
                      type: string
                      enum:
                      - "True"
                      - "False"
                      - Unknown
                    type:
                      description: type of condition in CamelCase or in foo.example.com/CamelCase.
                        --- Many .condition.type values are consistent across resources
                        like Available, but because arbitrary conditions can be useful
                        (see .node.status.conditions), the ability to deconflict is
                        important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
                      type: string
                      maxLength: 316
                      pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                x-kubernetes-list-map-keys:
                - type
                x-kubernetes-list-type: map
`
)

type ClusterlinkReplace struct {
	Namespace string
}
