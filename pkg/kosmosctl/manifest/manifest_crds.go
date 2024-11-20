package manifest

const (
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
                    \    Conditions []metav1.Condition ` + "`" + `json:\"conditions,omitempty\"
                    patchStrategy:\"merge\" patchMergeKey:\"type\" protobuf:\"bytes,1,rep,name=conditions\"` + "`" + `
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

const Cluster = `---
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
    - jsonPath: .spec.clusterLinkOptions.networkType
      name: NETWORK_TYPE
      type: string
    - jsonPath: .spec.clusterLinkOptions.ipFamily
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
              clusterLinkOptions:
                properties:
                  autodetectionMethod:
                    type: string
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
                  enable:
                    default: true
                    type: boolean
                  globalCIDRsMap:
                    additionalProperties:
                      type: string
                    type: object
                  ipFamily:
                    default: all
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
              clusterTreeOptions:
                properties:
                  enable:
                    default: true
                    type: boolean
                  leafModels:
                    description: LeafModels provide an api to arrange the member cluster
                      with some rules to pretend one or more leaf node
                    items:
                      properties:
                        labels:
                          additionalProperties:
                            type: string
                          description: Labels that will be setting in the pretended
                            Node labels
                          type: object
                        leafNodeName:
                          description: LeafNodeName defines leaf name If nil or empty,
                            the leaf node name will generate by controller and fill
                            in cluster link status
                          type: string
                        nodeSelector:
                          description: NodeSelector is a selector to select member
                            cluster nodes to pretend a leaf node in clusterTree.
                          properties:
                            labelSelector:
                              description: LabelSelector is a filter to select member
                                cluster nodes to pretend a leaf node in clusterTree
                                by labels. It will work on second level schedule on
                                pod create in member clusters.
                              properties:
                                matchExpressions:
                                  description: matchExpressions is a list of label
                                    selector requirements. The requirements are ANDed.
                                  items:
                                    description: A label selector requirement is a
                                      selector that contains values, a key, and an
                                      operator that relates the key and values.
                                    properties:
                                      key:
                                        description: key is the label key that the
                                          selector applies to.
                                        type: string
                                      operator:
                                        description: operator represents a key's relationship
                                          to a set of values. Valid operators are
                                          In, NotIn, Exists and DoesNotExist.
                                        type: string
                                      values:
                                        description: values is an array of string
                                          values. If the operator is In or NotIn,
                                          the values array must be non-empty. If the
                                          operator is Exists or DoesNotExist, the
                                          values array must be empty. This array is
                                          replaced during a strategic merge patch.
                                        items:
                                          type: string
                                        type: array
                                    required:
                                    - key
                                    - operator
                                    type: object
                                  type: array
                                matchLabels:
                                  additionalProperties:
                                    type: string
                                  description: matchLabels is a map of {key,value}
                                    pairs. A single {key,value} in the matchLabels
                                    map is equivalent to an element of matchExpressions,
                                    whose key field is "key", the operator is "In",
                                    and the values array contains only "value". The
                                    requirements are ANDed.
                                  type: object
                              type: object
                              x-kubernetes-map-type: atomic
                            nodeName:
                              description: NodeName is Member cluster origin node
                                Name
                              type: string
                          type: object
                        taints:
                          description: Taints attached to the leaf pretended Node.
                            If nil or empty, controller will set the default no-schedule
                            taint
                          items:
                            description: The node this Taint is attached to has the
                              "effect" on any pod that does not tolerate the Taint.
                            properties:
                              effect:
                                description: Required. The effect of the taint on
                                  pods that do not tolerate the taint. Valid effects
                                  are NoSchedule, PreferNoSchedule and NoExecute.
                                type: string
                              key:
                                description: Required. The taint key to be applied
                                  to a node.
                                type: string
                              timeAdded:
                                description: TimeAdded represents the time at which
                                  the taint was added. It is only written for NoExecute
                                  taints.
                                format: date-time
                                type: string
                              value:
                                description: The taint value corresponding to the
                                  taint key.
                                type: string
                            required:
                            - effect
                            - key
                            type: object
                          type: array
                      type: object
                    type: array
                type: object
              imageRepository:
                type: string
              kubeconfig:
                format: byte
                type: string
              namespace:
                default: kosmos-system
                type: string
            type: object
          status:
            description: Status describes the current status of a cluster.
            properties:
              clusterLinkStatus:
                description: ClusterLinkStatus contain the cluster network information
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
              clusterTreeStatus:
                description: ClusterTreeStatus contain the member cluster leafNode
                  end status
                properties:
                  leafNodeItems:
                    description: LeafNodeItems represents list of the leaf node Items
                      calculating in each member cluster.
                    items:
                      properties:
                        leafNodeName:
                          description: LeafNodeName represents the leaf node name
                            generate by controller. suggest name format like cluster-shortLabel-number
                            like member-az1-1
                          type: string
                      required:
                      - leafNodeName
                      type: object
                    type: array
                type: object
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources: {}
`

const ClusterlinkClusterNode = `---
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

const ClusterlinkNodeConfig = `---
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

const DaemonSet = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: daemonsets.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: DaemonSet
    listKind: DaemonSetList
    plural: daemonsets
    shortNames:
    - kdaemon
    - kds
    singular: daemonset
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: The desired number of pods.
      jsonPath: .status.desiredNumberScheduled
      name: DESIRED
      type: integer
    - description: The current number of pods.
      jsonPath: .status.currentNumberScheduled
      name: CURRENT
      type: integer
    - description: The ready number of pods.
      jsonPath: .status.numberReady
      name: READY
      type: integer
    - description: The updated number of pods.
      jsonPath: .status.updatedNumberScheduled
      name: UP-TO-DATE
      type: integer
    - description: The updated number of pods.
      jsonPath: .status.numberAvailable
      name: AVAILABLE
      type: integer
    - description: CreationTimestamp is a timestamp representing the server time when
        this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC.
      jsonPath: .metadata.creationTimestamp
      name: AGE
      type: date
    - description: The containers of currently  daemonset.
      jsonPath: .spec.template.spec.containers[*].name
      name: CONTAINERS
      priority: 1
      type: string
    - description: The images of currently advanced daemonset.
      jsonPath: .spec.template.spec.containers[*].image
      name: IMAGES
      priority: 1
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: DaemonSet represents the configuration of a daemon set.
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
            description: 'The desired behavior of this daemon set. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status'
            properties:
              minReadySeconds:
                description: The minimum number of seconds for which a newly created
                  DaemonSet pod should be ready without any of its container crashing,
                  for it to be considered available. Defaults to 0 (pod will be considered
                  available as soon as it is ready).
                format: int32
                type: integer
              revisionHistoryLimit:
                description: The number of old history to retain to allow rollback.
                  This is a pointer to distinguish between explicit zero and not specified.
                  Defaults to 10.
                format: int32
                type: integer
              selector:
                description: 'A label query over pods that are managed by the daemon
                  set. Must match in order to be controlled. It must match the pod
                  template''s labels. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors'
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
                x-kubernetes-map-type: atomic
              template:
                description: 'An object that describes the pod that will be created.
                  The DaemonSet will create exactly one copy of this pod on every
                  node that matches the template''s node selector (or on every node
                  if no node selector is specified). More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template'
                x-kubernetes-preserve-unknown-fields: true
              updateStrategy:
                description: An update strategy to replace existing DaemonSet pods
                  with new pods.
                properties:
                  rollingUpdate:
                    description: 'Rolling update config params. Present only if type
                      = "RollingUpdate". --- TODO: Update this to follow our convention
                      for oneOf, whatever we decide it to be. Same as Deployment ` + "`" + `strategy.rollingUpdate` + "`" + `.
                      See https://github.com/kubernetes/kubernetes/issues/35345'
                    properties:
                      maxSurge:
                        anyOf:
                        - type: integer
                        - type: string
                        description: 'The maximum number of nodes with an existing
                          available DaemonSet pod that can have an updated DaemonSet
                          pod during during an update. Value can be an absolute number
                          (ex: 5) or a percentage of desired pods (ex: 10%). This
                          can not be 0 if MaxUnavailable is 0. Absolute number is
                          calculated from percentage by rounding up to a minimum of
                          1. Default value is 0. Example: when this is set to 30%,
                          at most 30% of the total number of nodes that should be
                          running the daemon pod (i.e. status.desiredNumberScheduled)
                          can have their a new pod created before the old pod is marked
                          as deleted. The update starts by launching new pods on 30%
                          of nodes. Once an updated pod is available (Ready for at
                          least minReadySeconds) the old DaemonSet pod on that node
                          is marked deleted. If the old pod becomes unavailable for
                          any reason (Ready transitions to false, is evicted, or is
                          drained) an updated pod is immediatedly created on that
                          node without considering surge limits. Allowing surge implies
                          the possibility that the resources consumed by the daemonset
                          on any given node can double if the readiness check fails,
                          and so resource intensive daemonsets should take into account
                          that they may cause evictions during disruption.'
                        x-kubernetes-int-or-string: true
                      maxUnavailable:
                        anyOf:
                        - type: integer
                        - type: string
                        description: 'The maximum number of DaemonSet pods that can
                          be unavailable during the update. Value can be an absolute
                          number (ex: 5) or a percentage of total number of DaemonSet
                          pods at the start of the update (ex: 10%). Absolute number
                          is calculated from percentage by rounding up. This cannot
                          be 0 if MaxSurge is 0 Default value is 1. Example: when
                          this is set to 30%, at most 30% of the total number of nodes
                          that should be running the daemon pod (i.e. status.desiredNumberScheduled)
                          can have their pods stopped for an update at any given time.
                          The update starts by stopping at most 30% of those DaemonSet
                          pods and then brings up new DaemonSet pods in their place.
                          Once the new pods are available, it then proceeds onto other
                          DaemonSet pods, thus ensuring that at least 70% of original
                          number of DaemonSet pods are available at all times during
                          the update.'
                        x-kubernetes-int-or-string: true
                    type: object
                  type:
                    description: Type of daemon set update. Can be "RollingUpdate"
                      or "OnDelete". Default is RollingUpdate.
                    type: string
                type: object
            required:
            - selector
            - template
            type: object
          status:
            description: 'The current status of this daemon set. This data may be
              out of date by some window of time. Populated by the system. Read-only.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status'
            properties:
              collisionCount:
                description: Count of hash collisions for the DaemonSet. The DaemonSet
                  controller uses this field as a collision avoidance mechanism when
                  it needs to create the name for the newest ControllerRevision.
                format: int32
                type: integer
              conditions:
                description: Represents the latest available observations of a DaemonSet's
                  current state.
                items:
                  description: DaemonSetCondition describes the state of a DaemonSet
                    at a certain point.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of DaemonSet condition.
                      type: string
                  required:
                  - status
                  - type
                  type: object
                type: array
              currentNumberScheduled:
                description: 'The number of nodes that are running at least 1 daemon
                  pod and are supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              desiredNumberScheduled:
                description: 'The total number of nodes that should be running the
                  daemon pod (including nodes correctly running the daemon pod). More
                  info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              numberAvailable:
                description: The number of nodes that should be running the daemon
                  pod and have one or more of the daemon pod running and available
                  (ready for at least spec.minReadySeconds)
                format: int32
                type: integer
              numberMisscheduled:
                description: 'The number of nodes that are running the daemon pod,
                  but are not supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              numberReady:
                description: numberReady is the number of nodes that should be running
                  the daemon pod and have one or more of the daemon pod running with
                  a Ready Condition.
                format: int32
                type: integer
              numberUnavailable:
                description: The number of nodes that should be running the daemon
                  pod and have none of the daemon pod running and available (ready
                  for at least spec.minReadySeconds)
                format: int32
                type: integer
              observedGeneration:
                description: The most recent generation observed by the daemon set
                  controller.
                format: int64
                type: integer
              updatedNumberScheduled:
                description: The total number of nodes that are running updated daemon
                  pod
                format: int32
                type: integer
            required:
            - currentNumberScheduled
            - desiredNumberScheduled
            - numberMisscheduled
            - numberReady
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`

const ShadowDaemonSet = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: shadowdaemonsets.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: ShadowDaemonSet
    listKind: ShadowDaemonSetList
    plural: shadowdaemonsets
    shortNames:
    - ksds
    singular: shadowdaemonset
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: The desired number of pods.
      jsonPath: .status.desiredNumberScheduled
      name: DESIRED
      type: integer
    - description: The current number of pods.
      jsonPath: .status.currentNumberScheduled
      name: CURRENT
      type: integer
    - description: The ready number of pods.
      jsonPath: .status.numberReady
      name: READY
      type: integer
    - description: The updated number of pods.
      jsonPath: .status.updatedNumberScheduled
      name: UP-TO-DATE
      type: integer
    - description: The updated number of pods.
      jsonPath: .status.numberAvailable
      name: AVAILABLE
      type: integer
    - description: CreationTimestamp is a timestamp representing the server time when
        this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC.
      jsonPath: .metadata.creationTimestamp
      name: AGE
      type: date
    - description: The containers of currently  daemonset.
      jsonPath: .daemonSetSpec.template.spec.containers[*].name
      name: CONTAINERS
      priority: 1
      type: string
    - description: The images of currently advanced daemonset.
      jsonPath: .daemonSetSpec.template.spec.containers[*].image
      name: IMAGES
      priority: 1
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
          cluster:
            type: string
          daemonSetSpec:
            description: DaemonSetSpec is the specification of a daemon set.
            properties:
              minReadySeconds:
                description: The minimum number of seconds for which a newly created
                  DaemonSet pod should be ready without any of its container crashing,
                  for it to be considered available. Defaults to 0 (pod will be considered
                  available as soon as it is ready).
                format: int32
                type: integer
              revisionHistoryLimit:
                description: The number of old history to retain to allow rollback.
                  This is a pointer to distinguish between explicit zero and not specified.
                  Defaults to 10.
                format: int32
                type: integer
              selector:
                description: 'A label query over pods that are managed by the daemon
                  set. Must match in order to be controlled. It must match the pod
                  template''s labels. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors'
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
                x-kubernetes-map-type: atomic
              template:
                description: 'An object that describes the pod that will be created.
                  The DaemonSet will create exactly one copy of this pod on every
                  node that matches the template''s node selector (or on every node
                  if no node selector is specified). More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template'
                x-kubernetes-preserve-unknown-fields: true
              updateStrategy:
                description: An update strategy to replace existing DaemonSet pods
                  with new pods.
                properties:
                  rollingUpdate:
                    description: 'Rolling update config params. Present only if type
                      = "RollingUpdate". --- TODO: Update this to follow our convention
                      for oneOf, whatever we decide it to be. Same as Deployment ` + "`" + `strategy.rollingUpdate` + "`" + `.
                      See https://github.com/kubernetes/kubernetes/issues/35345'
                    properties:
                      maxSurge:
                        anyOf:
                        - type: integer
                        - type: string
                        description: 'The maximum number of nodes with an existing
                          available DaemonSet pod that can have an updated DaemonSet
                          pod during during an update. Value can be an absolute number
                          (ex: 5) or a percentage of desired pods (ex: 10%). This
                          can not be 0 if MaxUnavailable is 0. Absolute number is
                          calculated from percentage by rounding up to a minimum of
                          1. Default value is 0. Example: when this is set to 30%,
                          at most 30% of the total number of nodes that should be
                          running the daemon pod (i.e. status.desiredNumberScheduled)
                          can have their a new pod created before the old pod is marked
                          as deleted. The update starts by launching new pods on 30%
                          of nodes. Once an updated pod is available (Ready for at
                          least minReadySeconds) the old DaemonSet pod on that node
                          is marked deleted. If the old pod becomes unavailable for
                          any reason (Ready transitions to false, is evicted, or is
                          drained) an updated pod is immediatedly created on that
                          node without considering surge limits. Allowing surge implies
                          the possibility that the resources consumed by the daemonset
                          on any given node can double if the readiness check fails,
                          and so resource intensive daemonsets should take into account
                          that they may cause evictions during disruption.'
                        x-kubernetes-int-or-string: true
                      maxUnavailable:
                        anyOf:
                        - type: integer
                        - type: string
                        description: 'The maximum number of DaemonSet pods that can
                          be unavailable during the update. Value can be an absolute
                          number (ex: 5) or a percentage of total number of DaemonSet
                          pods at the start of the update (ex: 10%). Absolute number
                          is calculated from percentage by rounding up. This cannot
                          be 0 if MaxSurge is 0 Default value is 1. Example: when
                          this is set to 30%, at most 30% of the total number of nodes
                          that should be running the daemon pod (i.e. status.desiredNumberScheduled)
                          can have their pods stopped for an update at any given time.
                          The update starts by stopping at most 30% of those DaemonSet
                          pods and then brings up new DaemonSet pods in their place.
                          Once the new pods are available, it then proceeds onto other
                          DaemonSet pods, thus ensuring that at least 70% of original
                          number of DaemonSet pods are available at all times during
                          the update.'
                        x-kubernetes-int-or-string: true
                    type: object
                  type:
                    description: Type of daemon set update. Can be "RollingUpdate"
                      or "OnDelete". Default is RollingUpdate.
                    type: string
                type: object
            required:
            - selector
            - template
            type: object
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          refType:
            type: string
          status:
            description: DaemonSetStatus represents the current status of a daemon
              set.
            properties:
              collisionCount:
                description: Count of hash collisions for the DaemonSet. The DaemonSet
                  controller uses this field as a collision avoidance mechanism when
                  it needs to create the name for the newest ControllerRevision.
                format: int32
                type: integer
              conditions:
                description: Represents the latest available observations of a DaemonSet's
                  current state.
                items:
                  description: DaemonSetCondition describes the state of a DaemonSet
                    at a certain point.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of DaemonSet condition.
                      type: string
                  required:
                  - status
                  - type
                  type: object
                type: array
              currentNumberScheduled:
                description: 'The number of nodes that are running at least 1 daemon
                  pod and are supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              desiredNumberScheduled:
                description: 'The total number of nodes that should be running the
                  daemon pod (including nodes correctly running the daemon pod). More
                  info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              numberAvailable:
                description: The number of nodes that should be running the daemon
                  pod and have one or more of the daemon pod running and available
                  (ready for at least spec.minReadySeconds)
                format: int32
                type: integer
              numberMisscheduled:
                description: 'The number of nodes that are running the daemon pod,
                  but are not supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                format: int32
                type: integer
              numberReady:
                description: numberReady is the number of nodes that should be running
                  the daemon pod and have one or more of the daemon pod running with
                  a Ready Condition.
                format: int32
                type: integer
              numberUnavailable:
                description: The number of nodes that should be running the daemon
                  pod and have none of the daemon pod running and available (ready
                  for at least spec.minReadySeconds)
                format: int32
                type: integer
              observedGeneration:
                description: The most recent generation observed by the daemon set
                  controller.
                format: int64
                type: integer
              updatedNumberScheduled:
                description: The total number of nodes that are running updated daemon
                  pod
                format: int32
                type: integer
            required:
            - currentNumberScheduled
            - desiredNumberScheduled
            - numberMisscheduled
            - numberReady
            type: object
        required:
        - refType
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`

const ClusterPodConvert = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusterpodconvertpolicies.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: ClusterPodConvertPolicy
    listKind: ClusterPodConvertPolicyList
    plural: clusterpodconvertpolicies
    shortNames:
      - cpcp
    singular: clusterpodconvertpolicy
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
              description: Spec is the specification for the behaviour of the ClusterPodConvertPolicy.
              properties:
                converters:
                  description: Converters are some converter for convert pod when pod
                    synced from root cluster to leaf cluster pod will use these converters
                    to scheduled in leaf cluster
                  properties:
                    affinityConverter:
                      description: AffinityConverter used to modify the pod's Affinity
                        when pod synced to leaf cluster
                      properties:
                        affinity:
                          description: Affinity is a group of affinity scheduling rules.
                          properties:
                            nodeAffinity:
                              description: Describes node affinity scheduling rules
                                for the pod.
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling affinity expressions,
                                    etc.), compute a sum by iterating through the elements
                                    of this field and adding "weight" to the sum if
                                    the node matches the corresponding matchExpressions;
                                    the node(s) with the highest sum are the most preferred.
                                  items:
                                    description: An empty preferred scheduling term
                                      matches all objects with implicit weight 0 (i.e.
                                      it's a no-op). A null preferred scheduling term
                                      matches no objects (i.e. is also a no-op).
                                    properties:
                                      preference:
                                        description: A node selector term, associated
                                          with the corresponding weight.
                                        properties:
                                          matchExpressions:
                                            description: A list of node selector requirements
                                              by node's labels.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchFields:
                                            description: A list of node selector requirements
                                              by node's fields.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      weight:
                                        description: Weight associated with matching
                                          the corresponding nodeSelectorTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - preference
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    affinity requirements specified by this field cease
                                    to be met at some point during pod execution (e.g.
                                    due to an update), the system may or may not try
                                    to eventually evict the pod from its node.
                                  properties:
                                    nodeSelectorTerms:
                                      description: Required. A list of node selector
                                        terms. The terms are ORed.
                                      items:
                                        description: A null or empty node selector term
                                          matches no objects. The requirements of them
                                          are ANDed. The TopologySelectorTerm type implements
                                          a subset of the NodeSelectorTerm.
                                        properties:
                                          matchExpressions:
                                            description: A list of node selector requirements
                                              by node's labels.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchFields:
                                            description: A list of node selector requirements
                                              by node's fields.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      type: array
                                  required:
                                    - nodeSelectorTerms
                                  type: object
                                  x-kubernetes-map-type: atomic
                              type: object
                            podAffinity:
                              description: Describes pod affinity scheduling rules (e.g.
                                co-locate this pod in the same node, zone, etc. as some
                                other pod(s)).
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling affinity expressions,
                                    etc.), compute a sum by iterating through the elements
                                    of this field and adding "weight" to the sum if
                                    the node has pods which matches the corresponding
                                    podAffinityTerm; the node(s) with the highest sum
                                    are the most preferred.
                                  items:
                                    description: The weights of all of the matched WeightedPodAffinityTerm
                                      fields are added per-node to find the most preferred
                                      node(s)
                                    properties:
                                      podAffinityTerm:
                                        description: Required. A pod affinity term,
                                          associated with the corresponding weight.
                                        properties:
                                          labelSelector:
                                            description: A label query over a set of
                                              resources, in this case pods.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaceSelector:
                                            description: A label query over the set
                                              of namespaces that the term applies to.
                                              The term is applied to the union of the
                                              namespaces selected by this field and
                                              the ones listed in the namespaces field.
                                              null selector and null or empty namespaces
                                              list means "this pod's namespace". An
                                              empty selector ({}) matches all namespaces.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaces:
                                            description: namespaces specifies a static
                                              list of namespace names that the term
                                              applies to. The term is applied to the
                                              union of the namespaces listed in this
                                              field and the ones selected by namespaceSelector.
                                              null or empty namespaces list and null
                                              namespaceSelector means "this pod's namespace".
                                            items:
                                              type: string
                                            type: array
                                          topologyKey:
                                            description: This pod should be co-located
                                              (affinity) or not co-located (anti-affinity)
                                              with the pods matching the labelSelector
                                              in the specified namespaces, where co-located
                                              is defined as running on a node whose
                                              value of the label with key topologyKey
                                              matches that of any node on which any
                                              of the selected pods is running. Empty
                                              topologyKey is not allowed.
                                            type: string
                                        required:
                                          - topologyKey
                                        type: object
                                      weight:
                                        description: weight associated with matching
                                          the corresponding podAffinityTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - podAffinityTerm
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    affinity requirements specified by this field cease
                                    to be met at some point during pod execution (e.g.
                                    due to a pod label update), the system may or may
                                    not try to eventually evict the pod from its node.
                                    When there are multiple elements, the lists of nodes
                                    corresponding to each podAffinityTerm are intersected,
                                    i.e. all terms must be satisfied.
                                  items:
                                    description: Defines a set of pods (namely those
                                      matching the labelSelector relative to the given
                                      namespace(s)) that this pod should be co-located
                                      (affinity) or not co-located (anti-affinity) with,
                                      where co-located is defined as running on a node
                                      whose value of the label with key <topologyKey>
                                      matches that of any node on which a pod of the
                                      set of pods is running
                                    properties:
                                      labelSelector:
                                        description: A label query over a set of resources,
                                          in this case pods.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaceSelector:
                                        description: A label query over the set of namespaces
                                          that the term applies to. The term is applied
                                          to the union of the namespaces selected by
                                          this field and the ones listed in the namespaces
                                          field. null selector and null or empty namespaces
                                          list means "this pod's namespace". An empty
                                          selector ({}) matches all namespaces.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaces:
                                        description: namespaces specifies a static list
                                          of namespace names that the term applies to.
                                          The term is applied to the union of the namespaces
                                          listed in this field and the ones selected
                                          by namespaceSelector. null or empty namespaces
                                          list and null namespaceSelector means "this
                                          pod's namespace".
                                        items:
                                          type: string
                                        type: array
                                      topologyKey:
                                        description: This pod should be co-located (affinity)
                                          or not co-located (anti-affinity) with the
                                          pods matching the labelSelector in the specified
                                          namespaces, where co-located is defined as
                                          running on a node whose value of the label
                                          with key topologyKey matches that of any node
                                          on which any of the selected pods is running.
                                          Empty topologyKey is not allowed.
                                        type: string
                                    required:
                                      - topologyKey
                                    type: object
                                  type: array
                              type: object
                            podAntiAffinity:
                              description: Describes pod anti-affinity scheduling rules
                                (e.g. avoid putting this pod in the same node, zone,
                                etc. as some other pod(s)).
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the anti-affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling anti-affinity
                                    expressions, etc.), compute a sum by iterating through
                                    the elements of this field and adding "weight" to
                                    the sum if the node has pods which matches the corresponding
                                    podAffinityTerm; the node(s) with the highest sum
                                    are the most preferred.
                                  items:
                                    description: The weights of all of the matched WeightedPodAffinityTerm
                                      fields are added per-node to find the most preferred
                                      node(s)
                                    properties:
                                      podAffinityTerm:
                                        description: Required. A pod affinity term,
                                          associated with the corresponding weight.
                                        properties:
                                          labelSelector:
                                            description: A label query over a set of
                                              resources, in this case pods.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaceSelector:
                                            description: A label query over the set
                                              of namespaces that the term applies to.
                                              The term is applied to the union of the
                                              namespaces selected by this field and
                                              the ones listed in the namespaces field.
                                              null selector and null or empty namespaces
                                              list means "this pod's namespace". An
                                              empty selector ({}) matches all namespaces.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaces:
                                            description: namespaces specifies a static
                                              list of namespace names that the term
                                              applies to. The term is applied to the
                                              union of the namespaces listed in this
                                              field and the ones selected by namespaceSelector.
                                              null or empty namespaces list and null
                                              namespaceSelector means "this pod's namespace".
                                            items:
                                              type: string
                                            type: array
                                          topologyKey:
                                            description: This pod should be co-located
                                              (affinity) or not co-located (anti-affinity)
                                              with the pods matching the labelSelector
                                              in the specified namespaces, where co-located
                                              is defined as running on a node whose
                                              value of the label with key topologyKey
                                              matches that of any node on which any
                                              of the selected pods is running. Empty
                                              topologyKey is not allowed.
                                            type: string
                                        required:
                                          - topologyKey
                                        type: object
                                      weight:
                                        description: weight associated with matching
                                          the corresponding podAffinityTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - podAffinityTerm
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the anti-affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    anti-affinity requirements specified by this field
                                    cease to be met at some point during pod execution
                                    (e.g. due to a pod label update), the system may
                                    or may not try to eventually evict the pod from
                                    its node. When there are multiple elements, the
                                    lists of nodes corresponding to each podAffinityTerm
                                    are intersected, i.e. all terms must be satisfied.
                                  items:
                                    description: Defines a set of pods (namely those
                                      matching the labelSelector relative to the given
                                      namespace(s)) that this pod should be co-located
                                      (affinity) or not co-located (anti-affinity) with,
                                      where co-located is defined as running on a node
                                      whose value of the label with key <topologyKey>
                                      matches that of any node on which a pod of the
                                      set of pods is running
                                    properties:
                                      labelSelector:
                                        description: A label query over a set of resources,
                                          in this case pods.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaceSelector:
                                        description: A label query over the set of namespaces
                                          that the term applies to. The term is applied
                                          to the union of the namespaces selected by
                                          this field and the ones listed in the namespaces
                                          field. null selector and null or empty namespaces
                                          list means "this pod's namespace". An empty
                                          selector ({}) matches all namespaces.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaces:
                                        description: namespaces specifies a static list
                                          of namespace names that the term applies to.
                                          The term is applied to the union of the namespaces
                                          listed in this field and the ones selected
                                          by namespaceSelector. null or empty namespaces
                                          list and null namespaceSelector means "this
                                          pod's namespace".
                                        items:
                                          type: string
                                        type: array
                                      topologyKey:
                                        description: This pod should be co-located (affinity)
                                          or not co-located (anti-affinity) with the
                                          pods matching the labelSelector in the specified
                                          namespaces, where co-located is defined as
                                          running on a node whose value of the label
                                          with key topologyKey matches that of any node
                                          on which any of the selected pods is running.
                                          Empty topologyKey is not allowed.
                                        type: string
                                    required:
                                      - topologyKey
                                    type: object
                                  type: array
                              type: object
                          type: object
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                      required:
                        - convertType
                      type: object
                    hostAliasesConverter:
                      description: HostAliasesConverter is an optional list of hosts
                        and IPs that will be injected into the pod's hosts file if specified.
                        This is only valid for non-hostNetwork pods.
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        hostAliases:
                          items:
                            description: HostAlias holds the mapping between IP and
                              hostnames that will be injected as an entry in the pod's
                              hosts file.
                            properties:
                              hostnames:
                                description: Hostnames for the above IP address.
                                items:
                                  type: string
                                type: array
                              ip:
                                description: IP address of the host file entry.
                                type: string
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                    nodeNameConverter:
                      description: NodeNameConverter used to modify the pod's nodeName
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        nodeName:
                          type: string
                      required:
                        - convertType
                      type: object
                    nodeSelectorConverter:
                      description: NodeSelectorConverter used to modify the pod's NodeSelector
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        nodeSelector:
                          additionalProperties:
                            type: string
                          type: object
                      required:
                        - convertType
                      type: object
                    schedulerNameConverter:
                      description: SchedulerNameConverter used to modify the pod's nodeName
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        schedulerName:
                          type: string
                      required:
                        - convertType
                      type: object
                    tolerationConverter:
                      description: TolerationConverter used to modify the pod's Tolerations
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        tolerations:
                          items:
                            description: The pod this Toleration is attached to tolerates
                              any taint that matches the triple <key,value,effect> using
                              the matching operator <operator>.
                            properties:
                              effect:
                                description: Effect indicates the taint effect to match.
                                  Empty means match all taint effects. When specified,
                                  allowed values are NoSchedule, PreferNoSchedule and
                                  NoExecute.
                                type: string
                              key:
                                description: Key is the taint key that the toleration
                                  applies to. Empty means match all taint keys. If the
                                  key is empty, operator must be Exists; this combination
                                  means to match all values and all keys.
                                type: string
                              operator:
                                description: Operator represents a key's relationship
                                  to the value. Valid operators are Exists and Equal.
                                  Defaults to Equal. Exists is equivalent to wildcard
                                  for value, so that a pod can tolerate all taints of
                                  a particular category.
                                type: string
                              tolerationSeconds:
                                description: TolerationSeconds represents the period
                                  of time the toleration (which must be of effect NoExecute,
                                  otherwise this field is ignored) tolerates the taint.
                                  By default, it is not set, which means tolerate the
                                  taint forever (do not evict). Zero and negative values
                                  will be treated as 0 (evict immediately) by the system.
                                format: int64
                                type: integer
                              value:
                                description: Value is the taint value the toleration
                                  matches to. If the operator is Exists, the value should
                                  be empty, otherwise just a regular string.
                                type: string
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                    topologySpreadConstraintsConverter:
                      description: TopologySpreadConstraintsConverter used to modify
                        the pod's TopologySpreadConstraints when pod synced to leaf
                        cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        topologySpreadConstraints:
                          description: TopologySpreadConstraints describes how a group
                            of pods ought to spread across topology domains. Scheduler
                            will schedule pods in a way which abides by the constraints.
                            All topologySpreadConstraints are ANDed.
                          items:
                            description: TopologySpreadConstraint specifies how to spread
                              matching pods among the given topology.
                            properties:
                              labelSelector:
                                description: LabelSelector is used to find matching
                                  pods. Pods that match this label selector are counted
                                  to determine the number of pods in their corresponding
                                  topology domain.
                                properties:
                                  matchExpressions:
                                    description: matchExpressions is a list of label
                                      selector requirements. The requirements are ANDed.
                                    items:
                                      description: A label selector requirement is a
                                        selector that contains values, a key, and an
                                        operator that relates the key and values.
                                      properties:
                                        key:
                                          description: key is the label key that the
                                            selector applies to.
                                          type: string
                                        operator:
                                          description: operator represents a key's relationship
                                            to a set of values. Valid operators are
                                            In, NotIn, Exists and DoesNotExist.
                                          type: string
                                        values:
                                          description: values is an array of string
                                            values. If the operator is In or NotIn,
                                            the values array must be non-empty. If the
                                            operator is Exists or DoesNotExist, the
                                            values array must be empty. This array is
                                            replaced during a strategic merge patch.
                                          items:
                                            type: string
                                          type: array
                                      required:
                                        - key
                                        - operator
                                      type: object
                                    type: array
                                  matchLabels:
                                    additionalProperties:
                                      type: string
                                    description: matchLabels is a map of {key,value}
                                      pairs. A single {key,value} in the matchLabels
                                      map is equivalent to an element of matchExpressions,
                                      whose key field is "key", the operator is "In",
                                      and the values array contains only "value". The
                                      requirements are ANDed.
                                    type: object
                                type: object
                                x-kubernetes-map-type: atomic
                              matchLabelKeys:
                                description: MatchLabelKeys is a set of pod label keys
                                  to select the pods over which spreading will be calculated.
                                  The keys are used to lookup values from the incoming
                                  pod labels, those key-value labels are ANDed with
                                  labelSelector to select the group of existing pods
                                  over which spreading will be calculated for the incoming
                                  pod. Keys that don't exist in the incoming pod labels
                                  will be ignored. A null or empty list means only match
                                  against labelSelector.
                                items:
                                  type: string
                                type: array
                                x-kubernetes-list-type: atomic
                              maxSkew:
                                description: 'MaxSkew describes the degree to which
                                pods may be unevenly distributed. When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `,
                                it is the maximum permitted difference between the
                                number of matching pods in the target topology and
                                the global minimum. The global minimum is the minimum
                                number of matching pods in an eligible domain or zero
                                if the number of eligible domains is less than MinDomains.
                                For example, in a 3-zone cluster, MaxSkew is set to
                                1, and pods with the same labelSelector spread as
                                2/2/1: In this case, the global minimum is 1. | zone1
                                | zone2 | zone3 | |  P P  |  P P  |   P   | - if MaxSkew
                                is 1, incoming pod can only be scheduled to zone3
                                to become 2/2/2; scheduling it onto zone1(zone2) would
                                make the ActualSkew(3-1) on zone1(zone2) violate MaxSkew(1).
                                - if MaxSkew is 2, incoming pod can be scheduled onto
                                any zone. When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `,
                                it is used to give higher precedence to topologies
                                that satisfy it. It''s a required field. Default value
                                is 1 and 0 is not allowed.'
                                format: int32
                                type: integer
                              minDomains:
                                description: "MinDomains indicates a minimum number
                                of eligible domains. When the number of eligible domains
                                with matching topology keys is less than minDomains,
                                Pod Topology Spread treats \"global minimum\" as 0,
                                and then the calculation of Skew is performed. And
                                when the number of eligible domains with matching
                                topology keys equals or greater than minDomains, this
                                value has no effect on scheduling. As a result, when
                                the number of eligible domains is less than minDomains,
                                scheduler won't schedule more than maxSkew Pods to
                                those domains. If value is nil, the constraint behaves
                                as if MinDomains is equal to 1. Valid values are integers
                                greater than 0. When value is not nil, WhenUnsatisfiable
                                must be DoNotSchedule. \n For example, in a 3-zone
                                cluster, MaxSkew is set to 2, MinDomains is set to
                                5 and pods with the same labelSelector spread as 2/2/2:
                                | zone1 | zone2 | zone3 | |  P P  |  P P  |  P P  |
                                The number of domains is less than 5(MinDomains),
                                so \"global minimum\" is treated as 0. In this situation,
                                new pod with the same labelSelector cannot be scheduled,
                                because computed skew will be 3(3 - 0) if new Pod
                                is scheduled to any of the three zones, it will violate
                                MaxSkew. \n This is a beta field and requires the
                                MinDomainsInPodTopologySpread feature gate to be enabled
                                (enabled by default)."
                                format: int32
                                type: integer
                              nodeAffinityPolicy:
                                description: "NodeAffinityPolicy indicates how we will
                                treat Pod's nodeAffinity/nodeSelector when calculating
                                pod topology spread skew. Options are: - Honor: only
                                nodes matching nodeAffinity/nodeSelector are included
                                in the calculations. - Ignore: nodeAffinity/nodeSelector
                                are ignored. All nodes are included in the calculations.
                                \n If this value is nil, the behavior is equivalent
                                to the Honor policy. This is a beta-level feature
                                default enabled by the NodeInclusionPolicyInPodTopologySpread
                                feature flag."
                                type: string
                              nodeTaintsPolicy:
                                description: "NodeTaintsPolicy indicates how we will
                                treat node taints when calculating pod topology spread
                                skew. Options are: - Honor: nodes without taints,
                                along with tainted nodes for which the incoming pod
                                has a toleration, are included. - Ignore: node taints
                                are ignored. All nodes are included. \n If this value
                                is nil, the behavior is equivalent to the Ignore policy.
                                This is a beta-level feature default enabled by the
                                NodeInclusionPolicyInPodTopologySpread feature flag."
                                type: string
                              topologyKey:
                                description: TopologyKey is the key of node labels.
                                  Nodes that have a label with this key and identical
                                  values are considered to be in the same topology.
                                  We consider each <key, value> as a "bucket", and try
                                  to put balanced number of pods into each bucket. We
                                  define a domain as a particular instance of a topology.
                                  Also, we define an eligible domain as a domain whose
                                  nodes meet the requirements of nodeAffinityPolicy
                                  and nodeTaintsPolicy. e.g. If TopologyKey is "kubernetes.io/hostname",
                                  each Node is a domain of that topology. And, if TopologyKey
                                  is "topology.kubernetes.io/zone", each zone is a domain
                                  of that topology. It's a required field.
                                type: string
                              whenUnsatisfiable:
                                description: 'WhenUnsatisfiable indicates how to deal
                                with a pod if it doesn''t satisfy the spread constraint.
                                - DoNotSchedule (default) tells the scheduler not
                                to schedule it. - ScheduleAnyway tells the scheduler
                                to schedule the pod in any location, but giving higher
                                precedence to topologies that would help reduce the
                                skew. A constraint is considered "Unsatisfiable" for
                                an incoming pod if and only if every possible node
                                assignment for that pod would violate "MaxSkew" on
                                some topology. For example, in a 3-zone cluster, MaxSkew
                                is set to 1, and pods with the same labelSelector
                                spread as 3/1/1: | zone1 | zone2 | zone3 | | P P P
                                |   P   |   P   | If WhenUnsatisfiable is set to DoNotSchedule,
                                incoming pod can only be scheduled to zone2(zone3)
                                to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3)
                                satisfies MaxSkew(1). In other words, the cluster
                                can still be imbalanced, but scheduler won''t make
                                it *more* imbalanced. It''s a required field.'
                                type: string
                            required:
                              - maxSkew
                              - topologyKey
                              - whenUnsatisfiable
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                  type: object
                labelSelector:
                  description: A label query over a set of resources. If name is not
                    empty, labelSelector will be ignored.
                  properties:
                    matchExpressions:
                      description: matchExpressions is a list of label selector requirements.
                        The requirements are ANDed.
                      items:
                        description: A label selector requirement is a selector that
                          contains values, a key, and an operator that relates the key
                          and values.
                        properties:
                          key:
                            description: key is the label key that the selector applies
                              to.
                            type: string
                          operator:
                            description: operator represents a key's relationship to
                              a set of values. Valid operators are In, NotIn, Exists
                              and DoesNotExist.
                            type: string
                          values:
                            description: values is an array of string values. If the
                              operator is In or NotIn, the values array must be non-empty.
                              If the operator is Exists or DoesNotExist, the values
                              array must be empty. This array is replaced during a strategic
                              merge patch.
                            items:
                              type: string
                            type: array
                        required:
                          - key
                          - operator
                        type: object
                      type: array
                    matchLabels:
                      additionalProperties:
                        type: string
                      description: matchLabels is a map of {key,value} pairs. A single
                        {key,value} in the matchLabels map is equivalent to an element
                        of matchExpressions, whose key field is "key", the operator
                        is "In", and the values array contains only "value". The requirements
                        are ANDed.
                      type: object
                  type: object
                  x-kubernetes-map-type: atomic
                leafNodeSelector:
                  description: A label query over a set of resources. If name is not
                    empty, LeafNodeSelector will be ignored.
                  properties:
                    matchExpressions:
                      description: matchExpressions is a list of label selector requirements.
                        The requirements are ANDed.
                      items:
                        description: A label selector requirement is a selector that
                          contains values, a key, and an operator that relates the key
                          and values.
                        properties:
                          key:
                            description: key is the label key that the selector applies
                              to.
                            type: string
                          operator:
                            description: operator represents a key's relationship to
                              a set of values. Valid operators are In, NotIn, Exists
                              and DoesNotExist.
                            type: string
                          values:
                            description: values is an array of string values. If the
                              operator is In or NotIn, the values array must be non-empty.
                              If the operator is Exists or DoesNotExist, the values
                              array must be empty. This array is replaced during a strategic
                              merge patch.
                            items:
                              type: string
                            type: array
                        required:
                          - key
                          - operator
                        type: object
                      type: array
                    matchLabels:
                      additionalProperties:
                        type: string
                      description: matchLabels is a map of {key,value} pairs. A single
                        {key,value} in the matchLabels map is equivalent to an element
                        of matchExpressions, whose key field is "key", the operator
                        is "In", and the values array contains only "value". The requirements
                        are ANDed.
                      type: object
                  type: object
                  x-kubernetes-map-type: atomic
              required:
                - labelSelector
              type: object
          required:
            - spec
          type: object
      served: true
      storage: true
      subresources:
        status: {}
`

const PodConvert = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: podconvertpolicies.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: PodConvertPolicy
    listKind: PodConvertPolicyList
    plural: podconvertpolicies
    shortNames:
      - pcp
    singular: podconvertpolicy
  scope: Namespaced
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
              description: Spec is the specification for the behaviour of the PodConvertPolicy.
              properties:
                converters:
                  description: Converters are some converter for convert pod when pod
                    synced from root cluster to leaf cluster pod will use these converters
                    to scheduled in leaf cluster
                  properties:
                    affinityConverter:
                      description: AffinityConverter used to modify the pod's Affinity
                        when pod synced to leaf cluster
                      properties:
                        affinity:
                          description: Affinity is a group of affinity scheduling rules.
                          properties:
                            nodeAffinity:
                              description: Describes node affinity scheduling rules
                                for the pod.
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling affinity expressions,
                                    etc.), compute a sum by iterating through the elements
                                    of this field and adding "weight" to the sum if
                                    the node matches the corresponding matchExpressions;
                                    the node(s) with the highest sum are the most preferred.
                                  items:
                                    description: An empty preferred scheduling term
                                      matches all objects with implicit weight 0 (i.e.
                                      it's a no-op). A null preferred scheduling term
                                      matches no objects (i.e. is also a no-op).
                                    properties:
                                      preference:
                                        description: A node selector term, associated
                                          with the corresponding weight.
                                        properties:
                                          matchExpressions:
                                            description: A list of node selector requirements
                                              by node's labels.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchFields:
                                            description: A list of node selector requirements
                                              by node's fields.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      weight:
                                        description: Weight associated with matching
                                          the corresponding nodeSelectorTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - preference
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    affinity requirements specified by this field cease
                                    to be met at some point during pod execution (e.g.
                                    due to an update), the system may or may not try
                                    to eventually evict the pod from its node.
                                  properties:
                                    nodeSelectorTerms:
                                      description: Required. A list of node selector
                                        terms. The terms are ORed.
                                      items:
                                        description: A null or empty node selector term
                                          matches no objects. The requirements of them
                                          are ANDed. The TopologySelectorTerm type implements
                                          a subset of the NodeSelectorTerm.
                                        properties:
                                          matchExpressions:
                                            description: A list of node selector requirements
                                              by node's labels.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchFields:
                                            description: A list of node selector requirements
                                              by node's fields.
                                            items:
                                              description: A node selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: The label key that the
                                                    selector applies to.
                                                  type: string
                                                operator:
                                                  description: Represents a key's relationship
                                                    to a set of values. Valid operators
                                                    are In, NotIn, Exists, DoesNotExist.
                                                    Gt, and Lt.
                                                  type: string
                                                values:
                                                  description: An array of string values.
                                                    If the operator is In or NotIn,
                                                    the values array must be non-empty.
                                                    If the operator is Exists or DoesNotExist,
                                                    the values array must be empty.
                                                    If the operator is Gt or Lt, the
                                                    values array must have a single
                                                    element, which will be interpreted
                                                    as an integer. This array is replaced
                                                    during a strategic merge patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      type: array
                                  required:
                                    - nodeSelectorTerms
                                  type: object
                                  x-kubernetes-map-type: atomic
                              type: object
                            podAffinity:
                              description: Describes pod affinity scheduling rules (e.g.
                                co-locate this pod in the same node, zone, etc. as some
                                other pod(s)).
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling affinity expressions,
                                    etc.), compute a sum by iterating through the elements
                                    of this field and adding "weight" to the sum if
                                    the node has pods which matches the corresponding
                                    podAffinityTerm; the node(s) with the highest sum
                                    are the most preferred.
                                  items:
                                    description: The weights of all of the matched WeightedPodAffinityTerm
                                      fields are added per-node to find the most preferred
                                      node(s)
                                    properties:
                                      podAffinityTerm:
                                        description: Required. A pod affinity term,
                                          associated with the corresponding weight.
                                        properties:
                                          labelSelector:
                                            description: A label query over a set of
                                              resources, in this case pods.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaceSelector:
                                            description: A label query over the set
                                              of namespaces that the term applies to.
                                              The term is applied to the union of the
                                              namespaces selected by this field and
                                              the ones listed in the namespaces field.
                                              null selector and null or empty namespaces
                                              list means "this pod's namespace". An
                                              empty selector ({}) matches all namespaces.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaces:
                                            description: namespaces specifies a static
                                              list of namespace names that the term
                                              applies to. The term is applied to the
                                              union of the namespaces listed in this
                                              field and the ones selected by namespaceSelector.
                                              null or empty namespaces list and null
                                              namespaceSelector means "this pod's namespace".
                                            items:
                                              type: string
                                            type: array
                                          topologyKey:
                                            description: This pod should be co-located
                                              (affinity) or not co-located (anti-affinity)
                                              with the pods matching the labelSelector
                                              in the specified namespaces, where co-located
                                              is defined as running on a node whose
                                              value of the label with key topologyKey
                                              matches that of any node on which any
                                              of the selected pods is running. Empty
                                              topologyKey is not allowed.
                                            type: string
                                        required:
                                          - topologyKey
                                        type: object
                                      weight:
                                        description: weight associated with matching
                                          the corresponding podAffinityTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - podAffinityTerm
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    affinity requirements specified by this field cease
                                    to be met at some point during pod execution (e.g.
                                    due to a pod label update), the system may or may
                                    not try to eventually evict the pod from its node.
                                    When there are multiple elements, the lists of nodes
                                    corresponding to each podAffinityTerm are intersected,
                                    i.e. all terms must be satisfied.
                                  items:
                                    description: Defines a set of pods (namely those
                                      matching the labelSelector relative to the given
                                      namespace(s)) that this pod should be co-located
                                      (affinity) or not co-located (anti-affinity) with,
                                      where co-located is defined as running on a node
                                      whose value of the label with key <topologyKey>
                                      matches that of any node on which a pod of the
                                      set of pods is running
                                    properties:
                                      labelSelector:
                                        description: A label query over a set of resources,
                                          in this case pods.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaceSelector:
                                        description: A label query over the set of namespaces
                                          that the term applies to. The term is applied
                                          to the union of the namespaces selected by
                                          this field and the ones listed in the namespaces
                                          field. null selector and null or empty namespaces
                                          list means "this pod's namespace". An empty
                                          selector ({}) matches all namespaces.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaces:
                                        description: namespaces specifies a static list
                                          of namespace names that the term applies to.
                                          The term is applied to the union of the namespaces
                                          listed in this field and the ones selected
                                          by namespaceSelector. null or empty namespaces
                                          list and null namespaceSelector means "this
                                          pod's namespace".
                                        items:
                                          type: string
                                        type: array
                                      topologyKey:
                                        description: This pod should be co-located (affinity)
                                          or not co-located (anti-affinity) with the
                                          pods matching the labelSelector in the specified
                                          namespaces, where co-located is defined as
                                          running on a node whose value of the label
                                          with key topologyKey matches that of any node
                                          on which any of the selected pods is running.
                                          Empty topologyKey is not allowed.
                                        type: string
                                    required:
                                      - topologyKey
                                    type: object
                                  type: array
                              type: object
                            podAntiAffinity:
                              description: Describes pod anti-affinity scheduling rules
                                (e.g. avoid putting this pod in the same node, zone,
                                etc. as some other pod(s)).
                              properties:
                                preferredDuringSchedulingIgnoredDuringExecution:
                                  description: The scheduler will prefer to schedule
                                    pods to nodes that satisfy the anti-affinity expressions
                                    specified by this field, but it may choose a node
                                    that violates one or more of the expressions. The
                                    node that is most preferred is the one with the
                                    greatest sum of weights, i.e. for each node that
                                    meets all of the scheduling requirements (resource
                                    request, requiredDuringScheduling anti-affinity
                                    expressions, etc.), compute a sum by iterating through
                                    the elements of this field and adding "weight" to
                                    the sum if the node has pods which matches the corresponding
                                    podAffinityTerm; the node(s) with the highest sum
                                    are the most preferred.
                                  items:
                                    description: The weights of all of the matched WeightedPodAffinityTerm
                                      fields are added per-node to find the most preferred
                                      node(s)
                                    properties:
                                      podAffinityTerm:
                                        description: Required. A pod affinity term,
                                          associated with the corresponding weight.
                                        properties:
                                          labelSelector:
                                            description: A label query over a set of
                                              resources, in this case pods.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaceSelector:
                                            description: A label query over the set
                                              of namespaces that the term applies to.
                                              The term is applied to the union of the
                                              namespaces selected by this field and
                                              the ones listed in the namespaces field.
                                              null selector and null or empty namespaces
                                              list means "this pod's namespace". An
                                              empty selector ({}) matches all namespaces.
                                            properties:
                                              matchExpressions:
                                                description: matchExpressions is a list
                                                  of label selector requirements. The
                                                  requirements are ANDed.
                                                items:
                                                  description: A label selector requirement
                                                    is a selector that contains values,
                                                    a key, and an operator that relates
                                                    the key and values.
                                                  properties:
                                                    key:
                                                      description: key is the label
                                                        key that the selector applies
                                                        to.
                                                      type: string
                                                    operator:
                                                      description: operator represents
                                                        a key's relationship to a set
                                                        of values. Valid operators are
                                                        In, NotIn, Exists and DoesNotExist.
                                                      type: string
                                                    values:
                                                      description: values is an array
                                                        of string values. If the operator
                                                        is In or NotIn, the values array
                                                        must be non-empty. If the operator
                                                        is Exists or DoesNotExist, the
                                                        values array must be empty.
                                                        This array is replaced during
                                                        a strategic merge patch.
                                                      items:
                                                        type: string
                                                      type: array
                                                  required:
                                                    - key
                                                    - operator
                                                  type: object
                                                type: array
                                              matchLabels:
                                                additionalProperties:
                                                  type: string
                                                description: matchLabels is a map of
                                                  {key,value} pairs. A single {key,value}
                                                  in the matchLabels map is equivalent
                                                  to an element of matchExpressions,
                                                  whose key field is "key", the operator
                                                  is "In", and the values array contains
                                                  only "value". The requirements are
                                                  ANDed.
                                                type: object
                                            type: object
                                            x-kubernetes-map-type: atomic
                                          namespaces:
                                            description: namespaces specifies a static
                                              list of namespace names that the term
                                              applies to. The term is applied to the
                                              union of the namespaces listed in this
                                              field and the ones selected by namespaceSelector.
                                              null or empty namespaces list and null
                                              namespaceSelector means "this pod's namespace".
                                            items:
                                              type: string
                                            type: array
                                          topologyKey:
                                            description: This pod should be co-located
                                              (affinity) or not co-located (anti-affinity)
                                              with the pods matching the labelSelector
                                              in the specified namespaces, where co-located
                                              is defined as running on a node whose
                                              value of the label with key topologyKey
                                              matches that of any node on which any
                                              of the selected pods is running. Empty
                                              topologyKey is not allowed.
                                            type: string
                                        required:
                                          - topologyKey
                                        type: object
                                      weight:
                                        description: weight associated with matching
                                          the corresponding podAffinityTerm, in the
                                          range 1-100.
                                        format: int32
                                        type: integer
                                    required:
                                      - podAffinityTerm
                                      - weight
                                    type: object
                                  type: array
                                requiredDuringSchedulingIgnoredDuringExecution:
                                  description: If the anti-affinity requirements specified
                                    by this field are not met at scheduling time, the
                                    pod will not be scheduled onto the node. If the
                                    anti-affinity requirements specified by this field
                                    cease to be met at some point during pod execution
                                    (e.g. due to a pod label update), the system may
                                    or may not try to eventually evict the pod from
                                    its node. When there are multiple elements, the
                                    lists of nodes corresponding to each podAffinityTerm
                                    are intersected, i.e. all terms must be satisfied.
                                  items:
                                    description: Defines a set of pods (namely those
                                      matching the labelSelector relative to the given
                                      namespace(s)) that this pod should be co-located
                                      (affinity) or not co-located (anti-affinity) with,
                                      where co-located is defined as running on a node
                                      whose value of the label with key <topologyKey>
                                      matches that of any node on which a pod of the
                                      set of pods is running
                                    properties:
                                      labelSelector:
                                        description: A label query over a set of resources,
                                          in this case pods.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaceSelector:
                                        description: A label query over the set of namespaces
                                          that the term applies to. The term is applied
                                          to the union of the namespaces selected by
                                          this field and the ones listed in the namespaces
                                          field. null selector and null or empty namespaces
                                          list means "this pod's namespace". An empty
                                          selector ({}) matches all namespaces.
                                        properties:
                                          matchExpressions:
                                            description: matchExpressions is a list
                                              of label selector requirements. The requirements
                                              are ANDed.
                                            items:
                                              description: A label selector requirement
                                                is a selector that contains values,
                                                a key, and an operator that relates
                                                the key and values.
                                              properties:
                                                key:
                                                  description: key is the label key
                                                    that the selector applies to.
                                                  type: string
                                                operator:
                                                  description: operator represents a
                                                    key's relationship to a set of values.
                                                    Valid operators are In, NotIn, Exists
                                                    and DoesNotExist.
                                                  type: string
                                                values:
                                                  description: values is an array of
                                                    string values. If the operator is
                                                    In or NotIn, the values array must
                                                    be non-empty. If the operator is
                                                    Exists or DoesNotExist, the values
                                                    array must be empty. This array
                                                    is replaced during a strategic merge
                                                    patch.
                                                  items:
                                                    type: string
                                                  type: array
                                              required:
                                                - key
                                                - operator
                                              type: object
                                            type: array
                                          matchLabels:
                                            additionalProperties:
                                              type: string
                                            description: matchLabels is a map of {key,value}
                                              pairs. A single {key,value} in the matchLabels
                                              map is equivalent to an element of matchExpressions,
                                              whose key field is "key", the operator
                                              is "In", and the values array contains
                                              only "value". The requirements are ANDed.
                                            type: object
                                        type: object
                                        x-kubernetes-map-type: atomic
                                      namespaces:
                                        description: namespaces specifies a static list
                                          of namespace names that the term applies to.
                                          The term is applied to the union of the namespaces
                                          listed in this field and the ones selected
                                          by namespaceSelector. null or empty namespaces
                                          list and null namespaceSelector means "this
                                          pod's namespace".
                                        items:
                                          type: string
                                        type: array
                                      topologyKey:
                                        description: This pod should be co-located (affinity)
                                          or not co-located (anti-affinity) with the
                                          pods matching the labelSelector in the specified
                                          namespaces, where co-located is defined as
                                          running on a node whose value of the label
                                          with key topologyKey matches that of any node
                                          on which any of the selected pods is running.
                                          Empty topologyKey is not allowed.
                                        type: string
                                    required:
                                      - topologyKey
                                    type: object
                                  type: array
                              type: object
                          type: object
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                      required:
                        - convertType
                      type: object
                    hostAliasesConverter:
                      description: HostAliasesConverter is an optional list of hosts
                        and IPs that will be injected into the pod's hosts file if specified.
                        This is only valid for non-hostNetwork pods.
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        hostAliases:
                          items:
                            description: HostAlias holds the mapping between IP and
                              hostnames that will be injected as an entry in the pod's
                              hosts file.
                            properties:
                              hostnames:
                                description: Hostnames for the above IP address.
                                items:
                                  type: string
                                type: array
                              ip:
                                description: IP address of the host file entry.
                                type: string
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                    nodeNameConverter:
                      description: NodeNameConverter used to modify the pod's nodeName
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        nodeName:
                          type: string
                      required:
                        - convertType
                      type: object
                    nodeSelectorConverter:
                      description: NodeSelectorConverter used to modify the pod's NodeSelector
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        nodeSelector:
                          additionalProperties:
                            type: string
                          type: object
                      required:
                        - convertType
                      type: object
                    schedulerNameConverter:
                      description: SchedulerNameConverter used to modify the pod's nodeName
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        schedulerName:
                          type: string
                      required:
                        - convertType
                      type: object
                    tolerationConverter:
                      description: TolerationConverter used to modify the pod's Tolerations
                        when pod synced to leaf cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        tolerations:
                          items:
                            description: The pod this Toleration is attached to tolerates
                              any taint that matches the triple <key,value,effect> using
                              the matching operator <operator>.
                            properties:
                              effect:
                                description: Effect indicates the taint effect to match.
                                  Empty means match all taint effects. When specified,
                                  allowed values are NoSchedule, PreferNoSchedule and
                                  NoExecute.
                                type: string
                              key:
                                description: Key is the taint key that the toleration
                                  applies to. Empty means match all taint keys. If the
                                  key is empty, operator must be Exists; this combination
                                  means to match all values and all keys.
                                type: string
                              operator:
                                description: Operator represents a key's relationship
                                  to the value. Valid operators are Exists and Equal.
                                  Defaults to Equal. Exists is equivalent to wildcard
                                  for value, so that a pod can tolerate all taints of
                                  a particular category.
                                type: string
                              tolerationSeconds:
                                description: TolerationSeconds represents the period
                                  of time the toleration (which must be of effect NoExecute,
                                  otherwise this field is ignored) tolerates the taint.
                                  By default, it is not set, which means tolerate the
                                  taint forever (do not evict). Zero and negative values
                                  will be treated as 0 (evict immediately) by the system.
                                format: int64
                                type: integer
                              value:
                                description: Value is the taint value the toleration
                                  matches to. If the operator is Exists, the value should
                                  be empty, otherwise just a regular string.
                                type: string
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                    topologySpreadConstraintsConverter:
                      description: TopologySpreadConstraintsConverter used to modify
                        the pod's TopologySpreadConstraints when pod synced to leaf
                        cluster
                      properties:
                        convertType:
                          description: ConvertType if the operation type when convert
                            pod from root cluster to leaf cluster.
                          enum:
                            - add
                            - remove
                            - replace
                          type: string
                        topologySpreadConstraints:
                          description: TopologySpreadConstraints describes how a group
                            of pods ought to spread across topology domains. Scheduler
                            will schedule pods in a way which abides by the constraints.
                            All topologySpreadConstraints are ANDed.
                          items:
                            description: TopologySpreadConstraint specifies how to spread
                              matching pods among the given topology.
                            properties:
                              labelSelector:
                                description: LabelSelector is used to find matching
                                  pods. Pods that match this label selector are counted
                                  to determine the number of pods in their corresponding
                                  topology domain.
                                properties:
                                  matchExpressions:
                                    description: matchExpressions is a list of label
                                      selector requirements. The requirements are ANDed.
                                    items:
                                      description: A label selector requirement is a
                                        selector that contains values, a key, and an
                                        operator that relates the key and values.
                                      properties:
                                        key:
                                          description: key is the label key that the
                                            selector applies to.
                                          type: string
                                        operator:
                                          description: operator represents a key's relationship
                                            to a set of values. Valid operators are
                                            In, NotIn, Exists and DoesNotExist.
                                          type: string
                                        values:
                                          description: values is an array of string
                                            values. If the operator is In or NotIn,
                                            the values array must be non-empty. If the
                                            operator is Exists or DoesNotExist, the
                                            values array must be empty. This array is
                                            replaced during a strategic merge patch.
                                          items:
                                            type: string
                                          type: array
                                      required:
                                        - key
                                        - operator
                                      type: object
                                    type: array
                                  matchLabels:
                                    additionalProperties:
                                      type: string
                                    description: matchLabels is a map of {key,value}
                                      pairs. A single {key,value} in the matchLabels
                                      map is equivalent to an element of matchExpressions,
                                      whose key field is "key", the operator is "In",
                                      and the values array contains only "value". The
                                      requirements are ANDed.
                                    type: object
                                type: object
                                x-kubernetes-map-type: atomic
                              matchLabelKeys:
                                description: MatchLabelKeys is a set of pod label keys
                                  to select the pods over which spreading will be calculated.
                                  The keys are used to lookup values from the incoming
                                  pod labels, those key-value labels are ANDed with
                                  labelSelector to select the group of existing pods
                                  over which spreading will be calculated for the incoming
                                  pod. Keys that don't exist in the incoming pod labels
                                  will be ignored. A null or empty list means only match
                                  against labelSelector.
                                items:
                                  type: string
                                type: array
                                x-kubernetes-list-type: atomic
                              maxSkew:
                                description: 'MaxSkew describes the degree to which
                                pods may be unevenly distributed. When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `,
                                it is the maximum permitted difference between the
                                number of matching pods in the target topology and
                                the global minimum. The global minimum is the minimum
                                number of matching pods in an eligible domain or zero
                                if the number of eligible domains is less than MinDomains.
                                For example, in a 3-zone cluster, MaxSkew is set to
                                1, and pods with the same labelSelector spread as
                                2/2/1: In this case, the global minimum is 1. | zone1
                                | zone2 | zone3 | |  P P  |  P P  |   P   | - if MaxSkew
                                is 1, incoming pod can only be scheduled to zone3
                                to become 2/2/2; scheduling it onto zone1(zone2) would
                                make the ActualSkew(3-1) on zone1(zone2) violate MaxSkew(1).
                                - if MaxSkew is 2, incoming pod can be scheduled onto
                                any zone. When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `,
                                it is used to give higher precedence to topologies
                                that satisfy it. It''s a required field. Default value
                                is 1 and 0 is not allowed.'
                                format: int32
                                type: integer
                              minDomains:
                                description: "MinDomains indicates a minimum number
                                of eligible domains. When the number of eligible domains
                                with matching topology keys is less than minDomains,
                                Pod Topology Spread treats \"global minimum\" as 0,
                                and then the calculation of Skew is performed. And
                                when the number of eligible domains with matching
                                topology keys equals or greater than minDomains, this
                                value has no effect on scheduling. As a result, when
                                the number of eligible domains is less than minDomains,
                                scheduler won't schedule more than maxSkew Pods to
                                those domains. If value is nil, the constraint behaves
                                as if MinDomains is equal to 1. Valid values are integers
                                greater than 0. When value is not nil, WhenUnsatisfiable
                                must be DoNotSchedule. \n For example, in a 3-zone
                                cluster, MaxSkew is set to 2, MinDomains is set to
                                5 and pods with the same labelSelector spread as 2/2/2:
                                | zone1 | zone2 | zone3 | |  P P  |  P P  |  P P  |
                                The number of domains is less than 5(MinDomains),
                                so \"global minimum\" is treated as 0. In this situation,
                                new pod with the same labelSelector cannot be scheduled,
                                because computed skew will be 3(3 - 0) if new Pod
                                is scheduled to any of the three zones, it will violate
                                MaxSkew. \n This is a beta field and requires the
                                MinDomainsInPodTopologySpread feature gate to be enabled
                                (enabled by default)."
                                format: int32
                                type: integer
                              nodeAffinityPolicy:
                                description: "NodeAffinityPolicy indicates how we will
                                treat Pod's nodeAffinity/nodeSelector when calculating
                                pod topology spread skew. Options are: - Honor: only
                                nodes matching nodeAffinity/nodeSelector are included
                                in the calculations. - Ignore: nodeAffinity/nodeSelector
                                are ignored. All nodes are included in the calculations.
                                \n If this value is nil, the behavior is equivalent
                                to the Honor policy. This is a beta-level feature
                                default enabled by the NodeInclusionPolicyInPodTopologySpread
                                feature flag."
                                type: string
                              nodeTaintsPolicy:
                                description: "NodeTaintsPolicy indicates how we will
                                treat node taints when calculating pod topology spread
                                skew. Options are: - Honor: nodes without taints,
                                along with tainted nodes for which the incoming pod
                                has a toleration, are included. - Ignore: node taints
                                are ignored. All nodes are included. \n If this value
                                is nil, the behavior is equivalent to the Ignore policy.
                                This is a beta-level feature default enabled by the
                                NodeInclusionPolicyInPodTopologySpread feature flag."
                                type: string
                              topologyKey:
                                description: TopologyKey is the key of node labels.
                                  Nodes that have a label with this key and identical
                                  values are considered to be in the same topology.
                                  We consider each <key, value> as a "bucket", and try
                                  to put balanced number of pods into each bucket. We
                                  define a domain as a particular instance of a topology.
                                  Also, we define an eligible domain as a domain whose
                                  nodes meet the requirements of nodeAffinityPolicy
                                  and nodeTaintsPolicy. e.g. If TopologyKey is "kubernetes.io/hostname",
                                  each Node is a domain of that topology. And, if TopologyKey
                                  is "topology.kubernetes.io/zone", each zone is a domain
                                  of that topology. It's a required field.
                                type: string
                              whenUnsatisfiable:
                                description: 'WhenUnsatisfiable indicates how to deal
                                with a pod if it doesn''t satisfy the spread constraint.
                                - DoNotSchedule (default) tells the scheduler not
                                to schedule it. - ScheduleAnyway tells the scheduler
                                to schedule the pod in any location, but giving higher
                                precedence to topologies that would help reduce the
                                skew. A constraint is considered "Unsatisfiable" for
                                an incoming pod if and only if every possible node
                                assignment for that pod would violate "MaxSkew" on
                                some topology. For example, in a 3-zone cluster, MaxSkew
                                is set to 1, and pods with the same labelSelector
                                spread as 3/1/1: | zone1 | zone2 | zone3 | | P P P
                                |   P   |   P   | If WhenUnsatisfiable is set to DoNotSchedule,
                                incoming pod can only be scheduled to zone2(zone3)
                                to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3)
                                satisfies MaxSkew(1). In other words, the cluster
                                can still be imbalanced, but scheduler won''t make
                                it *more* imbalanced. It''s a required field.'
                                type: string
                            required:
                              - maxSkew
                              - topologyKey
                              - whenUnsatisfiable
                            type: object
                          type: array
                      required:
                        - convertType
                      type: object
                  type: object
                labelSelector:
                  description: A label query over a set of resources. If name is not
                    empty, labelSelector will be ignored.
                  properties:
                    matchExpressions:
                      description: matchExpressions is a list of label selector requirements.
                        The requirements are ANDed.
                      items:
                        description: A label selector requirement is a selector that
                          contains values, a key, and an operator that relates the key
                          and values.
                        properties:
                          key:
                            description: key is the label key that the selector applies
                              to.
                            type: string
                          operator:
                            description: operator represents a key's relationship to
                              a set of values. Valid operators are In, NotIn, Exists
                              and DoesNotExist.
                            type: string
                          values:
                            description: values is an array of string values. If the
                              operator is In or NotIn, the values array must be non-empty.
                              If the operator is Exists or DoesNotExist, the values
                              array must be empty. This array is replaced during a strategic
                              merge patch.
                            items:
                              type: string
                            type: array
                        required:
                          - key
                          - operator
                        type: object
                      type: array
                    matchLabels:
                      additionalProperties:
                        type: string
                      description: matchLabels is a map of {key,value} pairs. A single
                        {key,value} in the matchLabels map is equivalent to an element
                        of matchExpressions, whose key field is "key", the operator
                        is "In", and the values array contains only "value". The requirements
                        are ANDed.
                      type: object
                  type: object
                  x-kubernetes-map-type: atomic
                leafNodeSelector:
                  description: A label query over a set of resources. If name is not
                    empty, LeafNodeSelector will be ignored.
                  properties:
                    matchExpressions:
                      description: matchExpressions is a list of label selector requirements.
                        The requirements are ANDed.
                      items:
                        description: A label selector requirement is a selector that
                          contains values, a key, and an operator that relates the key
                          and values.
                        properties:
                          key:
                            description: key is the label key that the selector applies
                              to.
                            type: string
                          operator:
                            description: operator represents a key's relationship to
                              a set of values. Valid operators are In, NotIn, Exists
                              and DoesNotExist.
                            type: string
                          values:
                            description: values is an array of string values. If the
                              operator is In or NotIn, the values array must be non-empty.
                              If the operator is Exists or DoesNotExist, the values
                              array must be empty. This array is replaced during a strategic
                              merge patch.
                            items:
                              type: string
                            type: array
                        required:
                          - key
                          - operator
                        type: object
                      type: array
                    matchLabels:
                      additionalProperties:
                        type: string
                      description: matchLabels is a map of {key,value} pairs. A single
                        {key,value} in the matchLabels map is equivalent to an element
                        of matchExpressions, whose key field is "key", the operator
                        is "In", and the values array contains only "value". The requirements
                        are ANDed.
                      type: object
                  type: object
                  x-kubernetes-map-type: atomic
              required:
                - labelSelector
              type: object
          required:
            - spec
          type: object
      served: true
      storage: true
      subresources:
        status: {}
`

const SchedulerClusterDistributionPolicies = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusterdistributionpolicies.kosmos.io
spec:
  group: kosmos.io
  names:
    categories:
    - kosmos-io
    kind: ClusterDistributionPolicy
    listKind: ClusterDistributionPolicyList
    plural: clusterdistributionpolicies
    shortNames:
    - cdp
    singular: clusterdistributionpolicy
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - description: CreationTimestamp is a timestamp representing the server time when
        this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC.
      jsonPath: .metadata.creationTimestamp
      name: AGE
      type: date
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
            description: DistributionSpec represents the desired behavior of ClusterDistributionPolicy.
            properties:
              policyTerms:
                description: PolicyTerms represents the rule for select nodes to distribute
                  resources.
                items:
                  properties:
                    advancedTerm:
                      description: AdvancedTerm represents scheduling restrictions
                        to a certain set of nodes.
                      properties:
                        nodeName:
                          description: NodeName is a request to schedule this pod
                            onto a specific node. If it is non-empty, the scheduler
                            simply schedules this pod onto that node, assuming that
                            it fits resource requirements.
                          type: string
                        nodeSelector:
                          additionalProperties:
                            type: string
                          description: 'NodeSelector is a selector which must be true
                            for the pod to fit on a node. Selector which must match
                            a node''s labels for the pod to be scheduled on that node.
                            More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/'
                          type: object
                          x-kubernetes-map-type: atomic
                        tolerations:
                          description: If specified, the pod's tolerations.
                          items:
                            description: The pod this Toleration is attached to tolerates
                              any taint that matches the triple <key,value,effect>
                              using the matching operator <operator>.
                            properties:
                              effect:
                                description: Effect indicates the taint effect to
                                  match. Empty means match all taint effects. When
                                  specified, allowed values are NoSchedule, PreferNoSchedule
                                  and NoExecute.
                                type: string
                              key:
                                description: Key is the taint key that the toleration
                                  applies to. Empty means match all taint keys. If
                                  the key is empty, operator must be Exists; this
                                  combination means to match all values and all keys.
                                type: string
                              operator:
                                description: Operator represents a key's relationship
                                  to the value. Valid operators are Exists and Equal.
                                  Defaults to Equal. Exists is equivalent to wildcard
                                  for value, so that a pod can tolerate all taints
                                  of a particular category.
                                type: string
                              tolerationSeconds:
                                description: TolerationSeconds represents the period
                                  of time the toleration (which must be of effect
                                  NoExecute, otherwise this field is ignored) tolerates
                                  the taint. By default, it is not set, which means
                                  tolerate the taint forever (do not evict). Zero
                                  and negative values will be treated as 0 (evict
                                  immediately) by the system.
                                format: int64
                                type: integer
                              value:
                                description: Value is the taint value the toleration
                                  matches to. If the operator is Exists, the value
                                  should be empty, otherwise just a regular string.
                                type: string
                            type: object
                          type: array
                      type: object
                    name:
                      type: string
                    nodeType:
                      default: mix
                      description: NodeType declares the type for scheduling node.
                        Valid options are "host", "leaf", "mix", "adv".
                      enum:
                      - host
                      - leaf
                      - mix
                      - adv
                      type: string
                  required:
                  - name
                  type: object
                minItems: 1
                type: array
              resourceSelectors:
                description: ResourceSelectors used to select resources and is required.
                items:
                  description: ResourceSelector the resources will be selected.
                  properties:
                    labelSelector:
                      description: Filter resource by labelSelector If target resource
                        name is not empty, labelSelector will be ignored.
                      properties:
                        matchExpressions:
                          description: matchExpressions is a list of label selector
                            requirements. The requirements are ANDed.
                          items:
                            description: A label selector requirement is a selector
                              that contains values, a key, and an operator that relates
                              the key and values.
                            properties:
                              key:
                                description: key is the label key that the selector
                                  applies to.
                                type: string
                              operator:
                                description: operator represents a key's relationship
                                  to a set of values. Valid operators are In, NotIn,
                                  Exists and DoesNotExist.
                                type: string
                              values:
                                description: values is an array of string values.
                                  If the operator is In or NotIn, the values array
                                  must be non-empty. If the operator is Exists or
                                  DoesNotExist, the values array must be empty. This
                                  array is replaced during a strategic merge patch.
                                items:
                                  type: string
                                type: array
                            required:
                            - key
                            - operator
                            type: object
                          type: array
                        matchLabels:
                          additionalProperties:
                            type: string
                          description: matchLabels is a map of {key,value} pairs.
                            A single {key,value} in the matchLabels map is equivalent
                            to an element of matchExpressions, whose key field is
                            "key", the operator is "In", and the values array contains
                            only "value". The requirements are ANDed.
                          type: object
                      type: object
                      x-kubernetes-map-type: atomic
                    name:
                      description: Name of the target resource. Default is empty,
                        which means selecting all resources.
                      type: string
                    namePrefix:
                      description: NamePrefix the prefix of the target resource name
                      type: string
                    policyName:
                      description: Name of the Policy.
                      type: string
                  required:
                  - policyName
                  type: object
                minItems: 1
                type: array
            required:
            - policyTerms
            - resourceSelectors
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources: {}

`

const SchedulerDistributionPolicies = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: distributionpolicies.kosmos.io
spec:
  group: kosmos.io
  names:
    categories:
    - kosmos-io
    kind: DistributionPolicy
    listKind: DistributionPolicyList
    plural: distributionpolicies
    shortNames:
    - dp
    singular: distributionpolicy
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: CreationTimestamp is a timestamp representing the server time when
        this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC.
      jsonPath: .metadata.creationTimestamp
      name: AGE
      type: date
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
            description: DistributionSpec represents the desired behavior of DistributionPolicy.
            properties:
              policyTerms:
                description: PolicyTerms represents the rule for select nodes to distribute
                  resources.
                items:
                  properties:
                    advancedTerm:
                      description: AdvancedTerm represents scheduling restrictions
                        to a certain set of nodes.
                      properties:
                        nodeName:
                          description: NodeName is a request to schedule this pod
                            onto a specific node. If it is non-empty, the scheduler
                            simply schedules this pod onto that node, assuming that
                            it fits resource requirements.
                          type: string
                        nodeSelector:
                          additionalProperties:
                            type: string
                          description: 'NodeSelector is a selector which must be true
                            for the pod to fit on a node. Selector which must match
                            a node''s labels for the pod to be scheduled on that node.
                            More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/'
                          type: object
                          x-kubernetes-map-type: atomic
                        tolerations:
                          description: If specified, the pod's tolerations.
                          items:
                            description: The pod this Toleration is attached to tolerates
                              any taint that matches the triple <key,value,effect>
                              using the matching operator <operator>.
                            properties:
                              effect:
                                description: Effect indicates the taint effect to
                                  match. Empty means match all taint effects. When
                                  specified, allowed values are NoSchedule, PreferNoSchedule
                                  and NoExecute.
                                type: string
                              key:
                                description: Key is the taint key that the toleration
                                  applies to. Empty means match all taint keys. If
                                  the key is empty, operator must be Exists; this
                                  combination means to match all values and all keys.
                                type: string
                              operator:
                                description: Operator represents a key's relationship
                                  to the value. Valid operators are Exists and Equal.
                                  Defaults to Equal. Exists is equivalent to wildcard
                                  for value, so that a pod can tolerate all taints
                                  of a particular category.
                                type: string
                              tolerationSeconds:
                                description: TolerationSeconds represents the period
                                  of time the toleration (which must be of effect
                                  NoExecute, otherwise this field is ignored) tolerates
                                  the taint. By default, it is not set, which means
                                  tolerate the taint forever (do not evict). Zero
                                  and negative values will be treated as 0 (evict
                                  immediately) by the system.
                                format: int64
                                type: integer
                              value:
                                description: Value is the taint value the toleration
                                  matches to. If the operator is Exists, the value
                                  should be empty, otherwise just a regular string.
                                type: string
                            type: object
                          type: array
                      type: object
                    name:
                      type: string
                    nodeType:
                      default: mix
                      description: NodeType declares the type for scheduling node.
                        Valid options are "host", "leaf", "mix", "adv".
                      enum:
                      - host
                      - leaf
                      - mix
                      - adv
                      type: string
                  required:
                  - name
                  type: object
                minItems: 1
                type: array
              resourceSelectors:
                description: ResourceSelectors used to select resources and is required.
                items:
                  description: ResourceSelector the resources will be selected.
                  properties:
                    labelSelector:
                      description: Filter resource by labelSelector If target resource
                        name is not empty, labelSelector will be ignored.
                      properties:
                        matchExpressions:
                          description: matchExpressions is a list of label selector
                            requirements. The requirements are ANDed.
                          items:
                            description: A label selector requirement is a selector
                              that contains values, a key, and an operator that relates
                              the key and values.
                            properties:
                              key:
                                description: key is the label key that the selector
                                  applies to.
                                type: string
                              operator:
                                description: operator represents a key's relationship
                                  to a set of values. Valid operators are In, NotIn,
                                  Exists and DoesNotExist.
                                type: string
                              values:
                                description: values is an array of string values.
                                  If the operator is In or NotIn, the values array
                                  must be non-empty. If the operator is Exists or
                                  DoesNotExist, the values array must be empty. This
                                  array is replaced during a strategic merge patch.
                                items:
                                  type: string
                                type: array
                            required:
                            - key
                            - operator
                            type: object
                          type: array
                        matchLabels:
                          additionalProperties:
                            type: string
                          description: matchLabels is a map of {key,value} pairs.
                            A single {key,value} in the matchLabels map is equivalent
                            to an element of matchExpressions, whose key field is
                            "key", the operator is "In", and the values array contains
                            only "value". The requirements are ANDed.
                          type: object
                      type: object
                      x-kubernetes-map-type: atomic
                    name:
                      description: Name of the target resource. Default is empty,
                        which means selecting all resources.
                      type: string
                    namePrefix:
                      description: NamePrefix the prefix of the target resource name
                      type: string
                    policyName:
                      description: Name of the Policy.
                      type: string
                  required:
                  - policyName
                  type: object
                minItems: 1
                type: array
            required:
            - policyTerms
            - resourceSelectors
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources: {}

`

type CRDReplace struct {
	Namespace string
}

const PodConversionCRD = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: podconversions.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: PodConversion
    listKind: PodConversionList
    plural: podconversions
    shortNames:
    - pc
    - pcs
    singular: podconversion
  scope: Namespaced
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
            description: Spec is the specification for the behaviour of the podConversion.
            properties:
              converters:
                description: Converters are some converter for convert pod when pod
                  synced from root cluster to leaf cluster pod will use these converters
                  to scheduled in leaf cluster
                properties:
                  affinityConverter:
                    description: AffinityConverter used to modify the pod's Affinity
                      when pod synced to leaf cluster
                    properties:
                      affinity:
                        description: Affinity is a group of affinity scheduling rules.
                        properties:
                          nodeAffinity:
                            description: Describes node affinity scheduling rules
                              for the pod.
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: The scheduler will prefer to schedule
                                  pods to nodes that satisfy the affinity expressions
                                  specified by this field, but it may choose a node
                                  that violates one or more of the expressions. The
                                  node that is most preferred is the one with the
                                  greatest sum of weights, i.e. for each node that
                                  meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling affinity expressions,
                                  etc.), compute a sum by iterating through the elements
                                  of this field and adding "weight" to the sum if
                                  the node matches the corresponding matchExpressions;
                                  the node(s) with the highest sum are the most preferred.
                                items:
                                  description: An empty preferred scheduling term
                                    matches all objects with implicit weight 0 (i.e.
                                    it's a no-op). A null preferred scheduling term
                                    matches no objects (i.e. is also a no-op).
                                  properties:
                                    preference:
                                      description: A node selector term, associated
                                        with the corresponding weight.
                                      properties:
                                        matchExpressions:
                                          description: A list of node selector requirements
                                            by node's labels.
                                          items:
                                            description: A node selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: Represents a key's relationship
                                                  to a set of values. Valid operators
                                                  are In, NotIn, Exists, DoesNotExist.
                                                  Gt, and Lt.
                                                type: string
                                              values:
                                                description: An array of string values.
                                                  If the operator is In or NotIn,
                                                  the values array must be non-empty.
                                                  If the operator is Exists or DoesNotExist,
                                                  the values array must be empty.
                                                  If the operator is Gt or Lt, the
                                                  values array must have a single
                                                  element, which will be interpreted
                                                  as an integer. This array is replaced
                                                  during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchFields:
                                          description: A list of node selector requirements
                                            by node's fields.
                                          items:
                                            description: A node selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: Represents a key's relationship
                                                  to a set of values. Valid operators
                                                  are In, NotIn, Exists, DoesNotExist.
                                                  Gt, and Lt.
                                                type: string
                                              values:
                                                description: An array of string values.
                                                  If the operator is In or NotIn,
                                                  the values array must be non-empty.
                                                  If the operator is Exists or DoesNotExist,
                                                  the values array must be empty.
                                                  If the operator is Gt or Lt, the
                                                  values array must have a single
                                                  element, which will be interpreted
                                                  as an integer. This array is replaced
                                                  during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    weight:
                                      description: Weight associated with matching
                                        the corresponding nodeSelectorTerm, in the
                                        range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - preference
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: If the affinity requirements specified
                                  by this field are not met at scheduling time, the
                                  pod will not be scheduled onto the node. If the
                                  affinity requirements specified by this field cease
                                  to be met at some point during pod execution (e.g.
                                  due to an update), the system may or may not try
                                  to eventually evict the pod from its node.
                                properties:
                                  nodeSelectorTerms:
                                    description: Required. A list of node selector
                                      terms. The terms are ORed.
                                    items:
                                      description: A null or empty node selector term
                                        matches no objects. The requirements of them
                                        are ANDed. The TopologySelectorTerm type implements
                                        a subset of the NodeSelectorTerm.
                                      properties:
                                        matchExpressions:
                                          description: A list of node selector requirements
                                            by node's labels.
                                          items:
                                            description: A node selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: Represents a key's relationship
                                                  to a set of values. Valid operators
                                                  are In, NotIn, Exists, DoesNotExist.
                                                  Gt, and Lt.
                                                type: string
                                              values:
                                                description: An array of string values.
                                                  If the operator is In or NotIn,
                                                  the values array must be non-empty.
                                                  If the operator is Exists or DoesNotExist,
                                                  the values array must be empty.
                                                  If the operator is Gt or Lt, the
                                                  values array must have a single
                                                  element, which will be interpreted
                                                  as an integer. This array is replaced
                                                  during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchFields:
                                          description: A list of node selector requirements
                                            by node's fields.
                                          items:
                                            description: A node selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: Represents a key's relationship
                                                  to a set of values. Valid operators
                                                  are In, NotIn, Exists, DoesNotExist.
                                                  Gt, and Lt.
                                                type: string
                                              values:
                                                description: An array of string values.
                                                  If the operator is In or NotIn,
                                                  the values array must be non-empty.
                                                  If the operator is Exists or DoesNotExist,
                                                  the values array must be empty.
                                                  If the operator is Gt or Lt, the
                                                  values array must have a single
                                                  element, which will be interpreted
                                                  as an integer. This array is replaced
                                                  during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    type: array
                                required:
                                - nodeSelectorTerms
                                type: object
                                x-kubernetes-map-type: atomic
                            type: object
                          podAffinity:
                            description: Describes pod affinity scheduling rules (e.g.
                              co-locate this pod in the same node, zone, etc. as some
                              other pod(s)).
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: The scheduler will prefer to schedule
                                  pods to nodes that satisfy the affinity expressions
                                  specified by this field, but it may choose a node
                                  that violates one or more of the expressions. The
                                  node that is most preferred is the one with the
                                  greatest sum of weights, i.e. for each node that
                                  meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling affinity expressions,
                                  etc.), compute a sum by iterating through the elements
                                  of this field and adding "weight" to the sum if
                                  the node has pods which matches the corresponding
                                  podAffinityTerm; the node(s) with the highest sum
                                  are the most preferred.
                                items:
                                  description: The weights of all of the matched WeightedPodAffinityTerm
                                    fields are added per-node to find the most preferred
                                    node(s)
                                  properties:
                                    podAffinityTerm:
                                      description: Required. A pod affinity term,
                                        associated with the corresponding weight.
                                      properties:
                                        labelSelector:
                                          description: A label query over a set of
                                            resources, in this case pods.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: A label selector requirement
                                                  is a selector that contains values,
                                                  a key, and an operator that relates
                                                  the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: operator represents
                                                      a key's relationship to a set
                                                      of values. Valid operators are
                                                      In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: values is an array
                                                      of string values. If the operator
                                                      is In or NotIn, the values array
                                                      must be non-empty. If the operator
                                                      is Exists or DoesNotExist, the
                                                      values array must be empty.
                                                      This array is replaced during
                                                      a strategic merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: matchLabels is a map of
                                                {key,value} pairs. A single {key,value}
                                                in the matchLabels map is equivalent
                                                to an element of matchExpressions,
                                                whose key field is "key", the operator
                                                is "In", and the values array contains
                                                only "value". The requirements are
                                                ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaceSelector:
                                          description: A label query over the set
                                            of namespaces that the term applies to.
                                            The term is applied to the union of the
                                            namespaces selected by this field and
                                            the ones listed in the namespaces field.
                                            null selector and null or empty namespaces
                                            list means "this pod's namespace". An
                                            empty selector ({}) matches all namespaces.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: A label selector requirement
                                                  is a selector that contains values,
                                                  a key, and an operator that relates
                                                  the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: operator represents
                                                      a key's relationship to a set
                                                      of values. Valid operators are
                                                      In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: values is an array
                                                      of string values. If the operator
                                                      is In or NotIn, the values array
                                                      must be non-empty. If the operator
                                                      is Exists or DoesNotExist, the
                                                      values array must be empty.
                                                      This array is replaced during
                                                      a strategic merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: matchLabels is a map of
                                                {key,value} pairs. A single {key,value}
                                                in the matchLabels map is equivalent
                                                to an element of matchExpressions,
                                                whose key field is "key", the operator
                                                is "In", and the values array contains
                                                only "value". The requirements are
                                                ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaces:
                                          description: namespaces specifies a static
                                            list of namespace names that the term
                                            applies to. The term is applied to the
                                            union of the namespaces listed in this
                                            field and the ones selected by namespaceSelector.
                                            null or empty namespaces list and null
                                            namespaceSelector means "this pod's namespace".
                                          items:
                                            type: string
                                          type: array
                                        topologyKey:
                                          description: This pod should be co-located
                                            (affinity) or not co-located (anti-affinity)
                                            with the pods matching the labelSelector
                                            in the specified namespaces, where co-located
                                            is defined as running on a node whose
                                            value of the label with key topologyKey
                                            matches that of any node on which any
                                            of the selected pods is running. Empty
                                            topologyKey is not allowed.
                                          type: string
                                      required:
                                      - topologyKey
                                      type: object
                                    weight:
                                      description: weight associated with matching
                                        the corresponding podAffinityTerm, in the
                                        range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - podAffinityTerm
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: If the affinity requirements specified
                                  by this field are not met at scheduling time, the
                                  pod will not be scheduled onto the node. If the
                                  affinity requirements specified by this field cease
                                  to be met at some point during pod execution (e.g.
                                  due to a pod label update), the system may or may
                                  not try to eventually evict the pod from its node.
                                  When there are multiple elements, the lists of nodes
                                  corresponding to each podAffinityTerm are intersected,
                                  i.e. all terms must be satisfied.
                                items:
                                  description: Defines a set of pods (namely those
                                    matching the labelSelector relative to the given
                                    namespace(s)) that this pod should be co-located
                                    (affinity) or not co-located (anti-affinity) with,
                                    where co-located is defined as running on a node
                                    whose value of the label with key <topologyKey>
                                    matches that of any node on which a pod of the
                                    set of pods is running
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaceSelector:
                                      description: A label query over the set of namespaces
                                        that the term applies to. The term is applied
                                        to the union of the namespaces selected by
                                        this field and the ones listed in the namespaces
                                        field. null selector and null or empty namespaces
                                        list means "this pod's namespace". An empty
                                        selector ({}) matches all namespaces.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaces:
                                      description: namespaces specifies a static list
                                        of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces
                                        listed in this field and the ones selected
                                        by namespaceSelector. null or empty namespaces
                                        list and null namespaceSelector means "this
                                        pod's namespace".
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: This pod should be co-located (affinity)
                                        or not co-located (anti-affinity) with the
                                        pods matching the labelSelector in the specified
                                        namespaces, where co-located is defined as
                                        running on a node whose value of the label
                                        with key topologyKey matches that of any node
                                        on which any of the selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                type: array
                            type: object
                          podAntiAffinity:
                            description: Describes pod anti-affinity scheduling rules
                              (e.g. avoid putting this pod in the same node, zone,
                              etc. as some other pod(s)).
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: The scheduler will prefer to schedule
                                  pods to nodes that satisfy the anti-affinity expressions
                                  specified by this field, but it may choose a node
                                  that violates one or more of the expressions. The
                                  node that is most preferred is the one with the
                                  greatest sum of weights, i.e. for each node that
                                  meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling anti-affinity
                                  expressions, etc.), compute a sum by iterating through
                                  the elements of this field and adding "weight" to
                                  the sum if the node has pods which matches the corresponding
                                  podAffinityTerm; the node(s) with the highest sum
                                  are the most preferred.
                                items:
                                  description: The weights of all of the matched WeightedPodAffinityTerm
                                    fields are added per-node to find the most preferred
                                    node(s)
                                  properties:
                                    podAffinityTerm:
                                      description: Required. A pod affinity term,
                                        associated with the corresponding weight.
                                      properties:
                                        labelSelector:
                                          description: A label query over a set of
                                            resources, in this case pods.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: A label selector requirement
                                                  is a selector that contains values,
                                                  a key, and an operator that relates
                                                  the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: operator represents
                                                      a key's relationship to a set
                                                      of values. Valid operators are
                                                      In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: values is an array
                                                      of string values. If the operator
                                                      is In or NotIn, the values array
                                                      must be non-empty. If the operator
                                                      is Exists or DoesNotExist, the
                                                      values array must be empty.
                                                      This array is replaced during
                                                      a strategic merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: matchLabels is a map of
                                                {key,value} pairs. A single {key,value}
                                                in the matchLabels map is equivalent
                                                to an element of matchExpressions,
                                                whose key field is "key", the operator
                                                is "In", and the values array contains
                                                only "value". The requirements are
                                                ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaceSelector:
                                          description: A label query over the set
                                            of namespaces that the term applies to.
                                            The term is applied to the union of the
                                            namespaces selected by this field and
                                            the ones listed in the namespaces field.
                                            null selector and null or empty namespaces
                                            list means "this pod's namespace". An
                                            empty selector ({}) matches all namespaces.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: A label selector requirement
                                                  is a selector that contains values,
                                                  a key, and an operator that relates
                                                  the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: operator represents
                                                      a key's relationship to a set
                                                      of values. Valid operators are
                                                      In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: values is an array
                                                      of string values. If the operator
                                                      is In or NotIn, the values array
                                                      must be non-empty. If the operator
                                                      is Exists or DoesNotExist, the
                                                      values array must be empty.
                                                      This array is replaced during
                                                      a strategic merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: matchLabels is a map of
                                                {key,value} pairs. A single {key,value}
                                                in the matchLabels map is equivalent
                                                to an element of matchExpressions,
                                                whose key field is "key", the operator
                                                is "In", and the values array contains
                                                only "value". The requirements are
                                                ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaces:
                                          description: namespaces specifies a static
                                            list of namespace names that the term
                                            applies to. The term is applied to the
                                            union of the namespaces listed in this
                                            field and the ones selected by namespaceSelector.
                                            null or empty namespaces list and null
                                            namespaceSelector means "this pod's namespace".
                                          items:
                                            type: string
                                          type: array
                                        topologyKey:
                                          description: This pod should be co-located
                                            (affinity) or not co-located (anti-affinity)
                                            with the pods matching the labelSelector
                                            in the specified namespaces, where co-located
                                            is defined as running on a node whose
                                            value of the label with key topologyKey
                                            matches that of any node on which any
                                            of the selected pods is running. Empty
                                            topologyKey is not allowed.
                                          type: string
                                      required:
                                      - topologyKey
                                      type: object
                                    weight:
                                      description: weight associated with matching
                                        the corresponding podAffinityTerm, in the
                                        range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - podAffinityTerm
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: If the anti-affinity requirements specified
                                  by this field are not met at scheduling time, the
                                  pod will not be scheduled onto the node. If the
                                  anti-affinity requirements specified by this field
                                  cease to be met at some point during pod execution
                                  (e.g. due to a pod label update), the system may
                                  or may not try to eventually evict the pod from
                                  its node. When there are multiple elements, the
                                  lists of nodes corresponding to each podAffinityTerm
                                  are intersected, i.e. all terms must be satisfied.
                                items:
                                  description: Defines a set of pods (namely those
                                    matching the labelSelector relative to the given
                                    namespace(s)) that this pod should be co-located
                                    (affinity) or not co-located (anti-affinity) with,
                                    where co-located is defined as running on a node
                                    whose value of the label with key <topologyKey>
                                    matches that of any node on which a pod of the
                                    set of pods is running
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaceSelector:
                                      description: A label query over the set of namespaces
                                        that the term applies to. The term is applied
                                        to the union of the namespaces selected by
                                        this field and the ones listed in the namespaces
                                        field. null selector and null or empty namespaces
                                        list means "this pod's namespace". An empty
                                        selector ({}) matches all namespaces.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaces:
                                      description: namespaces specifies a static list
                                        of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces
                                        listed in this field and the ones selected
                                        by namespaceSelector. null or empty namespaces
                                        list and null namespaceSelector means "this
                                        pod's namespace".
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: This pod should be co-located (affinity)
                                        or not co-located (anti-affinity) with the
                                        pods matching the labelSelector in the specified
                                        namespaces, where co-located is defined as
                                        running on a node whose value of the label
                                        with key topologyKey matches that of any node
                                        on which any of the selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                type: array
                            type: object
                        type: object
                      conversionType:
                        description: ConversionType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        type: string
                    required:
                    - conversionType
                    type: object
                  nodeNameConverter:
                    description: NodeNameConverter used to modify the pod's nodeName
                      when pod synced to leaf cluster
                    properties:
                      conversionType:
                        description: ConversionType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        type: string
                      nodeName:
                        type: string
                    required:
                    - conversionType
                    type: object
                  nodeSelectorConverter:
                    description: NodeSelectorConverter used to modify the pod's NodeSelector
                      when pod synced to leaf cluster
                    properties:
                      conversionType:
                        description: ConversionType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      nodeSelector:
                        additionalProperties:
                          type: string
                        type: object
                    required:
                    - conversionType
                    type: object
                  schedulerNameConverter:
                    description: SchedulerNameConverter used to modify the pod's nodeName
                      when pod synced to leaf cluster
                    properties:
                      conversionType:
                        description: ConversionType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        type: string
                      schedulerName:
                        type: string
                    required:
                    - conversionType
                    type: object
                  topologySpreadConstraintsConverter:
                    description: TopologySpreadConstraintsConverter used to modify
                      the pod's TopologySpreadConstraints when pod synced to leaf
                      cluster
                    properties:
                      conversionType:
                        description: ConversionType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        type: string
                      topologySpreadConstraints:
                        description: TopologySpreadConstraints describes how a group
                          of pods ought to spread across topology domains. Scheduler
                          will schedule pods in a way which abides by the constraints.
                          All topologySpreadConstraints are ANDed.
                        items:
                          description: TopologySpreadConstraint specifies how to spread
                            matching pods among the given topology.
                          properties:
                            labelSelector:
                              description: LabelSelector is used to find matching
                                pods. Pods that match this label selector are counted
                                to determine the number of pods in their corresponding
                                topology domain.
                              properties:
                                matchExpressions:
                                  description: matchExpressions is a list of label
                                    selector requirements. The requirements are ANDed.
                                  items:
                                    description: A label selector requirement is a
                                      selector that contains values, a key, and an
                                      operator that relates the key and values.
                                    properties:
                                      key:
                                        description: key is the label key that the
                                          selector applies to.
                                        type: string
                                      operator:
                                        description: operator represents a key's relationship
                                          to a set of values. Valid operators are
                                          In, NotIn, Exists and DoesNotExist.
                                        type: string
                                      values:
                                        description: values is an array of string
                                          values. If the operator is In or NotIn,
                                          the values array must be non-empty. If the
                                          operator is Exists or DoesNotExist, the
                                          values array must be empty. This array is
                                          replaced during a strategic merge patch.
                                        items:
                                          type: string
                                        type: array
                                    required:
                                    - key
                                    - operator
                                    type: object
                                  type: array
                                matchLabels:
                                  additionalProperties:
                                    type: string
                                  description: matchLabels is a map of {key,value}
                                    pairs. A single {key,value} in the matchLabels
                                    map is equivalent to an element of matchExpressions,
                                    whose key field is "key", the operator is "In",
                                    and the values array contains only "value". The
                                    requirements are ANDed.
                                  type: object
                              type: object
                              x-kubernetes-map-type: atomic
                            matchLabelKeys:
                              description: MatchLabelKeys is a set of pod label keys
                                to select the pods over which spreading will be calculated.
                                The keys are used to lookup values from the incoming
                                pod labels, those key-value labels are ANDed with
                                labelSelector to select the group of existing pods
                                over which spreading will be calculated for the incoming
                                pod. Keys that don't exist in the incoming pod labels
                                will be ignored. A null or empty list means only match
                                against labelSelector.
                              items:
                                type: string
                              type: array
                              x-kubernetes-list-type: atomic
                            maxSkew:
                              description: 'MaxSkew describes the degree to which
                                pods may be unevenly distributed. When ` + "`" + `whenUnsatisfiable=DoNotSchedule` + "`" + `,
                                it is the maximum permitted difference between the
                                number of matching pods in the target topology and
                                the global minimum. The global minimum is the minimum
                                number of matching pods in an eligible domain or zero
                                if the number of eligible domains is less than MinDomains.
                                For example, in a 3-zone cluster, MaxSkew is set to
                                1, and pods with the same labelSelector spread as
                                2/2/1: In this case, the global minimum is 1. | zone1
                                | zone2 | zone3 | |  P P  |  P P  |   P   | - if MaxSkew
                                is 1, incoming pod can only be scheduled to zone3
                                to become 2/2/2; scheduling it onto zone1(zone2) would
                                make the ActualSkew(3-1) on zone1(zone2) violate MaxSkew(1).
                                - if MaxSkew is 2, incoming pod can be scheduled onto
                                any zone. When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `,
                                it is used to give higher precedence to topologies
                                that satisfy it. It''s a required field. Default value
                                is 1 and 0 is not allowed.'
                              format: int32
                              type: integer
                            minDomains:
                              description: "MinDomains indicates a minimum number
                                of eligible domains. When the number of eligible domains
                                with matching topology keys is less than minDomains,
                                Pod Topology Spread treats \"global minimum\" as 0,
                                and then the calculation of Skew is performed. And
                                when the number of eligible domains with matching
                                topology keys equals or greater than minDomains, this
                                value has no effect on scheduling. As a result, when
                                the number of eligible domains is less than minDomains,
                                scheduler won't schedule more than maxSkew Pods to
                                those domains. If value is nil, the constraint behaves
                                as if MinDomains is equal to 1. Valid values are integers
                                greater than 0. When value is not nil, WhenUnsatisfiable
                                must be DoNotSchedule. \n For example, in a 3-zone
                                cluster, MaxSkew is set to 2, MinDomains is set to
                                5 and pods with the same labelSelector spread as 2/2/2:
                                | zone1 | zone2 | zone3 | |  P P  |  P P  |  P P  |
                                The number of domains is less than 5(MinDomains),
                                so \"global minimum\" is treated as 0. In this situation,
                                new pod with the same labelSelector cannot be scheduled,
                                because computed skew will be 3(3 - 0) if new Pod
                                is scheduled to any of the three zones, it will violate
                                MaxSkew. \n This is a beta field and requires the
                                MinDomainsInPodTopologySpread feature gate to be enabled
                                (enabled by default)."
                              format: int32
                              type: integer
                            nodeAffinityPolicy:
                              description: "NodeAffinityPolicy indicates how we will
                                treat Pod's nodeAffinity/nodeSelector when calculating
                                pod topology spread skew. Options are: - Honor: only
                                nodes matching nodeAffinity/nodeSelector are included
                                in the calculations. - Ignore: nodeAffinity/nodeSelector
                                are ignored. All nodes are included in the calculations.
                                \n If this value is nil, the behavior is equivalent
                                to the Honor policy. This is a beta-level feature
                                default enabled by the NodeInclusionPolicyInPodTopologySpread
                                feature flag."
                              type: string
                            nodeTaintsPolicy:
                              description: "NodeTaintsPolicy indicates how we will
                                treat node taints when calculating pod topology spread
                                skew. Options are: - Honor: nodes without taints,
                                along with tainted nodes for which the incoming pod
                                has a toleration, are included. - Ignore: node taints
                                are ignored. All nodes are included. \n If this value
                                is nil, the behavior is equivalent to the Ignore policy.
                                This is a beta-level feature default enabled by the
                                NodeInclusionPolicyInPodTopologySpread feature flag."
                              type: string
                            topologyKey:
                              description: TopologyKey is the key of node labels.
                                Nodes that have a label with this key and identical
                                values are considered to be in the same topology.
                                We consider each <key, value> as a "bucket", and try
                                to put balanced number of pods into each bucket. We
                                define a domain as a particular instance of a topology.
                                Also, we define an eligible domain as a domain whose
                                nodes meet the requirements of nodeAffinityPolicy
                                and nodeTaintsPolicy. e.g. If TopologyKey is "kubernetes.io/hostname",
                                each Node is a domain of that topology. And, if TopologyKey
                                is "topology.kubernetes.io/zone", each zone is a domain
                                of that topology. It's a required field.
                              type: string
                            whenUnsatisfiable:
                              description: 'WhenUnsatisfiable indicates how to deal
                                with a pod if it doesn''t satisfy the spread constraint.
                                - DoNotSchedule (default) tells the scheduler not
                                to schedule it. - ScheduleAnyway tells the scheduler
                                to schedule the pod in any location, but giving higher
                                precedence to topologies that would help reduce the
                                skew. A constraint is considered "Unsatisfiable" for
                                an incoming pod if and only if every possible node
                                assignment for that pod would violate "MaxSkew" on
                                some topology. For example, in a 3-zone cluster, MaxSkew
                                is set to 1, and pods with the same labelSelector
                                spread as 3/1/1: | zone1 | zone2 | zone3 | | P P P
                                |   P   |   P   | If WhenUnsatisfiable is set to DoNotSchedule,
                                incoming pod can only be scheduled to zone2(zone3)
                                to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3)
                                satisfies MaxSkew(1). In other words, the cluster
                                can still be imbalanced, but scheduler won''t make
                                it *more* imbalanced. It''s a required field.'
                              type: string
                          required:
                          - maxSkew
                          - topologyKey
                          - whenUnsatisfiable
                          type: object
                        type: array
                    required:
                    - conversionType
                    type: object
                type: object
              labelSelector:
                description: A label query over a set of resources. If name is not
                  empty, labelSelector will be ignored.
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
                x-kubernetes-map-type: atomic
            required:
            - labelSelector
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
const PodConvertPolicyCRD = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.14.0
  name: podconvertpolicies.kosmos.io
spec:
  group: kosmos.io
  names:
    kind: PodConvertPolicy
    listKind: PodConvertPolicyList
    plural: podconvertpolicies
    shortNames:
    - pc
    singular: podconvertpolicy
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: |-
              APIVersion defines the versioned schema of this representation of an object.
              Servers should convert recognized schemas to the latest internal value, and
              may reject unrecognized values.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
            type: string
          kind:
            description: |-
              Kind is a string value representing the REST resource this object represents.
              Servers may infer this from the endpoint the client submits requests to.
              Cannot be updated.
              In CamelCase.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
            type: string
          metadata:
            type: object
          spec:
            description: Spec is the specification for the behaviour of the PodConvertPolicy.
            properties:
              converters:
                description: |-
                  Converters are some converter for convert pod when pod synced from root cluster to leaf cluster
                  pod will use these converters to scheduled in leaf cluster
                properties:
                  affinityConverter:
                    description: AffinityConverter used to modify the pod's Affinity
                      when pod synced to leaf cluster
                    properties:
                      affinity:
                        description: Affinity is a group of affinity scheduling rules.
                        properties:
                          nodeAffinity:
                            description: Describes node affinity scheduling rules
                              for the pod.
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  The scheduler will prefer to schedule pods to nodes that satisfy
                                  the affinity expressions specified by this field, but it may choose
                                  a node that violates one or more of the expressions. The node that is
                                  most preferred is the one with the greatest sum of weights, i.e.
                                  for each node that meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling affinity expressions, etc.),
                                  compute a sum by iterating through the elements of this field and adding
                                  "weight" to the sum if the node matches the corresponding matchExpressions; the
                                  node(s) with the highest sum are the most preferred.
                                items:
                                  description: |-
                                    An empty preferred scheduling term matches all objects with implicit weight 0
                                    (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).
                                  properties:
                                    preference:
                                      description: A node selector term, associated
                                        with the corresponding weight.
                                      properties:
                                        matchExpressions:
                                          description: A list of node selector requirements
                                            by node's labels.
                                          items:
                                            description: |-
                                              A node selector requirement is a selector that contains values, a key, and an operator
                                              that relates the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  Represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
                                                type: string
                                              values:
                                                description: |-
                                                  An array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. If the operator is Gt or Lt, the values
                                                  array must have a single element, which will be interpreted as an integer.
                                                  This array is replaced during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchFields:
                                          description: A list of node selector requirements
                                            by node's fields.
                                          items:
                                            description: |-
                                              A node selector requirement is a selector that contains values, a key, and an operator
                                              that relates the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  Represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
                                                type: string
                                              values:
                                                description: |-
                                                  An array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. If the operator is Gt or Lt, the values
                                                  array must have a single element, which will be interpreted as an integer.
                                                  This array is replaced during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    weight:
                                      description: Weight associated with matching
                                        the corresponding nodeSelectorTerm, in the
                                        range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - preference
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  If the affinity requirements specified by this field are not met at
                                  scheduling time, the pod will not be scheduled onto the node.
                                  If the affinity requirements specified by this field cease to be met
                                  at some point during pod execution (e.g. due to an update), the system
                                  may or may not try to eventually evict the pod from its node.
                                properties:
                                  nodeSelectorTerms:
                                    description: Required. A list of node selector
                                      terms. The terms are ORed.
                                    items:
                                      description: |-
                                        A null or empty node selector term matches no objects. The requirements of
                                        them are ANDed.
                                        The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.
                                      properties:
                                        matchExpressions:
                                          description: A list of node selector requirements
                                            by node's labels.
                                          items:
                                            description: |-
                                              A node selector requirement is a selector that contains values, a key, and an operator
                                              that relates the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  Represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
                                                type: string
                                              values:
                                                description: |-
                                                  An array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. If the operator is Gt or Lt, the values
                                                  array must have a single element, which will be interpreted as an integer.
                                                  This array is replaced during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchFields:
                                          description: A list of node selector requirements
                                            by node's fields.
                                          items:
                                            description: |-
                                              A node selector requirement is a selector that contains values, a key, and an operator
                                              that relates the key and values.
                                            properties:
                                              key:
                                                description: The label key that the
                                                  selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  Represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
                                                type: string
                                              values:
                                                description: |-
                                                  An array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. If the operator is Gt or Lt, the values
                                                  array must have a single element, which will be interpreted as an integer.
                                                  This array is replaced during a strategic merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    type: array
                                required:
                                - nodeSelectorTerms
                                type: object
                                x-kubernetes-map-type: atomic
                            type: object
                          podAffinity:
                            description: Describes pod affinity scheduling rules (e.g.
                              co-locate this pod in the same node, zone, etc. as some
                              other pod(s)).
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  The scheduler will prefer to schedule pods to nodes that satisfy
                                  the affinity expressions specified by this field, but it may choose
                                  a node that violates one or more of the expressions. The node that is
                                  most preferred is the one with the greatest sum of weights, i.e.
                                  for each node that meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling affinity expressions, etc.),
                                  compute a sum by iterating through the elements of this field and adding
                                  "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the
                                  node(s) with the highest sum are the most preferred.
                                items:
                                  description: The weights of all of the matched WeightedPodAffinityTerm
                                    fields are added per-node to find the most preferred
                                    node(s)
                                  properties:
                                    podAffinityTerm:
                                      description: Required. A pod affinity term,
                                        associated with the corresponding weight.
                                      properties:
                                        labelSelector:
                                          description: A label query over a set of
                                            resources, in this case pods.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: |-
                                                  A label selector requirement is a selector that contains values, a key, and an operator that
                                                  relates the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: |-
                                                      operator represents a key's relationship to a set of values.
                                                      Valid operators are In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: |-
                                                      values is an array of string values. If the operator is In or NotIn,
                                                      the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                      the values array must be empty. This array is replaced during a strategic
                                                      merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: |-
                                                matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                                map is equivalent to an element of matchExpressions, whose key field is "key", the
                                                operator is "In", and the values array contains only "value". The requirements are ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaceSelector:
                                          description: |-
                                            A label query over the set of namespaces that the term applies to.
                                            The term is applied to the union of the namespaces selected by this field
                                            and the ones listed in the namespaces field.
                                            null selector and null or empty namespaces list means "this pod's namespace".
                                            An empty selector ({}) matches all namespaces.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: |-
                                                  A label selector requirement is a selector that contains values, a key, and an operator that
                                                  relates the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: |-
                                                      operator represents a key's relationship to a set of values.
                                                      Valid operators are In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: |-
                                                      values is an array of string values. If the operator is In or NotIn,
                                                      the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                      the values array must be empty. This array is replaced during a strategic
                                                      merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: |-
                                                matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                                map is equivalent to an element of matchExpressions, whose key field is "key", the
                                                operator is "In", and the values array contains only "value". The requirements are ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaces:
                                          description: |-
                                            namespaces specifies a static list of namespace names that the term applies to.
                                            The term is applied to the union of the namespaces listed in this field
                                            and the ones selected by namespaceSelector.
                                            null or empty namespaces list and null namespaceSelector means "this pod's namespace".
                                          items:
                                            type: string
                                          type: array
                                        topologyKey:
                                          description: |-
                                            This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
                                            the labelSelector in the specified namespaces, where co-located is defined as running on a node
                                            whose value of the label with key topologyKey matches that of any node on which any of the
                                            selected pods is running.
                                            Empty topologyKey is not allowed.
                                          type: string
                                      required:
                                      - topologyKey
                                      type: object
                                    weight:
                                      description: |-
                                        weight associated with matching the corresponding podAffinityTerm,
                                        in the range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - podAffinityTerm
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  If the affinity requirements specified by this field are not met at
                                  scheduling time, the pod will not be scheduled onto the node.
                                  If the affinity requirements specified by this field cease to be met
                                  at some point during pod execution (e.g. due to a pod label update), the
                                  system may or may not try to eventually evict the pod from its node.
                                  When there are multiple elements, the lists of nodes corresponding to each
                                  podAffinityTerm are intersected, i.e. all terms must be satisfied.
                                items:
                                  description: |-
                                    Defines a set of pods (namely those matching the labelSelector
                                    relative to the given namespace(s)) that this pod should be
                                    co-located (affinity) or not co-located (anti-affinity) with,
                                    where co-located is defined as running on a node whose value of
                                    the label with key <topologyKey> matches that of any node on which
                                    a pod of the set of pods is running
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: |-
                                              A label selector requirement is a selector that contains values, a key, and an operator that
                                              relates the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  operator represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists and DoesNotExist.
                                                type: string
                                              values:
                                                description: |-
                                                  values is an array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. This array is replaced during a strategic
                                                  merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: |-
                                            matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions, whose key field is "key", the
                                            operator is "In", and the values array contains only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaceSelector:
                                      description: |-
                                        A label query over the set of namespaces that the term applies to.
                                        The term is applied to the union of the namespaces selected by this field
                                        and the ones listed in the namespaces field.
                                        null selector and null or empty namespaces list means "this pod's namespace".
                                        An empty selector ({}) matches all namespaces.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: |-
                                              A label selector requirement is a selector that contains values, a key, and an operator that
                                              relates the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  operator represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists and DoesNotExist.
                                                type: string
                                              values:
                                                description: |-
                                                  values is an array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. This array is replaced during a strategic
                                                  merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: |-
                                            matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions, whose key field is "key", the
                                            operator is "In", and the values array contains only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaces:
                                      description: |-
                                        namespaces specifies a static list of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces listed in this field
                                        and the ones selected by namespaceSelector.
                                        null or empty namespaces list and null namespaceSelector means "this pod's namespace".
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: |-
                                        This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
                                        the labelSelector in the specified namespaces, where co-located is defined as running on a node
                                        whose value of the label with key topologyKey matches that of any node on which any of the
                                        selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                type: array
                            type: object
                          podAntiAffinity:
                            description: Describes pod anti-affinity scheduling rules
                              (e.g. avoid putting this pod in the same node, zone,
                              etc. as some other pod(s)).
                            properties:
                              preferredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  The scheduler will prefer to schedule pods to nodes that satisfy
                                  the anti-affinity expressions specified by this field, but it may choose
                                  a node that violates one or more of the expressions. The node that is
                                  most preferred is the one with the greatest sum of weights, i.e.
                                  for each node that meets all of the scheduling requirements (resource
                                  request, requiredDuringScheduling anti-affinity expressions, etc.),
                                  compute a sum by iterating through the elements of this field and adding
                                  "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the
                                  node(s) with the highest sum are the most preferred.
                                items:
                                  description: The weights of all of the matched WeightedPodAffinityTerm
                                    fields are added per-node to find the most preferred
                                    node(s)
                                  properties:
                                    podAffinityTerm:
                                      description: Required. A pod affinity term,
                                        associated with the corresponding weight.
                                      properties:
                                        labelSelector:
                                          description: A label query over a set of
                                            resources, in this case pods.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: |-
                                                  A label selector requirement is a selector that contains values, a key, and an operator that
                                                  relates the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: |-
                                                      operator represents a key's relationship to a set of values.
                                                      Valid operators are In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: |-
                                                      values is an array of string values. If the operator is In or NotIn,
                                                      the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                      the values array must be empty. This array is replaced during a strategic
                                                      merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: |-
                                                matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                                map is equivalent to an element of matchExpressions, whose key field is "key", the
                                                operator is "In", and the values array contains only "value". The requirements are ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaceSelector:
                                          description: |-
                                            A label query over the set of namespaces that the term applies to.
                                            The term is applied to the union of the namespaces selected by this field
                                            and the ones listed in the namespaces field.
                                            null selector and null or empty namespaces list means "this pod's namespace".
                                            An empty selector ({}) matches all namespaces.
                                          properties:
                                            matchExpressions:
                                              description: matchExpressions is a list
                                                of label selector requirements. The
                                                requirements are ANDed.
                                              items:
                                                description: |-
                                                  A label selector requirement is a selector that contains values, a key, and an operator that
                                                  relates the key and values.
                                                properties:
                                                  key:
                                                    description: key is the label
                                                      key that the selector applies
                                                      to.
                                                    type: string
                                                  operator:
                                                    description: |-
                                                      operator represents a key's relationship to a set of values.
                                                      Valid operators are In, NotIn, Exists and DoesNotExist.
                                                    type: string
                                                  values:
                                                    description: |-
                                                      values is an array of string values. If the operator is In or NotIn,
                                                      the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                      the values array must be empty. This array is replaced during a strategic
                                                      merge patch.
                                                    items:
                                                      type: string
                                                    type: array
                                                required:
                                                - key
                                                - operator
                                                type: object
                                              type: array
                                            matchLabels:
                                              additionalProperties:
                                                type: string
                                              description: |-
                                                matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                                map is equivalent to an element of matchExpressions, whose key field is "key", the
                                                operator is "In", and the values array contains only "value". The requirements are ANDed.
                                              type: object
                                          type: object
                                          x-kubernetes-map-type: atomic
                                        namespaces:
                                          description: |-
                                            namespaces specifies a static list of namespace names that the term applies to.
                                            The term is applied to the union of the namespaces listed in this field
                                            and the ones selected by namespaceSelector.
                                            null or empty namespaces list and null namespaceSelector means "this pod's namespace".
                                          items:
                                            type: string
                                          type: array
                                        topologyKey:
                                          description: |-
                                            This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
                                            the labelSelector in the specified namespaces, where co-located is defined as running on a node
                                            whose value of the label with key topologyKey matches that of any node on which any of the
                                            selected pods is running.
                                            Empty topologyKey is not allowed.
                                          type: string
                                      required:
                                      - topologyKey
                                      type: object
                                    weight:
                                      description: |-
                                        weight associated with matching the corresponding podAffinityTerm,
                                        in the range 1-100.
                                      format: int32
                                      type: integer
                                  required:
                                  - podAffinityTerm
                                  - weight
                                  type: object
                                type: array
                              requiredDuringSchedulingIgnoredDuringExecution:
                                description: |-
                                  If the anti-affinity requirements specified by this field are not met at
                                  scheduling time, the pod will not be scheduled onto the node.
                                  If the anti-affinity requirements specified by this field cease to be met
                                  at some point during pod execution (e.g. due to a pod label update), the
                                  system may or may not try to eventually evict the pod from its node.
                                  When there are multiple elements, the lists of nodes corresponding to each
                                  podAffinityTerm are intersected, i.e. all terms must be satisfied.
                                items:
                                  description: |-
                                    Defines a set of pods (namely those matching the labelSelector
                                    relative to the given namespace(s)) that this pod should be
                                    co-located (affinity) or not co-located (anti-affinity) with,
                                    where co-located is defined as running on a node whose value of
                                    the label with key <topologyKey> matches that of any node on which
                                    a pod of the set of pods is running
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: |-
                                              A label selector requirement is a selector that contains values, a key, and an operator that
                                              relates the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  operator represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists and DoesNotExist.
                                                type: string
                                              values:
                                                description: |-
                                                  values is an array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. This array is replaced during a strategic
                                                  merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: |-
                                            matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions, whose key field is "key", the
                                            operator is "In", and the values array contains only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaceSelector:
                                      description: |-
                                        A label query over the set of namespaces that the term applies to.
                                        The term is applied to the union of the namespaces selected by this field
                                        and the ones listed in the namespaces field.
                                        null selector and null or empty namespaces list means "this pod's namespace".
                                        An empty selector ({}) matches all namespaces.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: |-
                                              A label selector requirement is a selector that contains values, a key, and an operator that
                                              relates the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: |-
                                                  operator represents a key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists and DoesNotExist.
                                                type: string
                                              values:
                                                description: |-
                                                  values is an array of string values. If the operator is In or NotIn,
                                                  the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                                  the values array must be empty. This array is replaced during a strategic
                                                  merge patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: |-
                                            matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions, whose key field is "key", the
                                            operator is "In", and the values array contains only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                      x-kubernetes-map-type: atomic
                                    namespaces:
                                      description: |-
                                        namespaces specifies a static list of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces listed in this field
                                        and the ones selected by namespaceSelector.
                                        null or empty namespaces list and null namespaceSelector means "this pod's namespace".
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: |-
                                        This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
                                        the labelSelector in the specified namespaces, where co-located is defined as running on a node
                                        whose value of the label with key topologyKey matches that of any node on which any of the
                                        selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                type: array
                            type: object
                        type: object
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                    required:
                    - convertType
                    type: object
                  nodeNameConverter:
                    description: NodeNameConverter used to modify the pod's nodeName
                      when pod synced to leaf cluster
                    properties:
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      nodeName:
                        type: string
                    required:
                    - convertType
                    type: object
                  nodeSelectorConverter:
                    description: NodeSelectorConverter used to modify the pod's NodeSelector
                      when pod synced to leaf cluster
                    properties:
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      nodeSelector:
                        additionalProperties:
                          type: string
                        type: object
                    required:
                    - convertType
                    type: object
                  schedulerNameConverter:
                    description: SchedulerNameConverter used to modify the pod's nodeName
                      when pod synced to leaf cluster
                    properties:
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      schedulerName:
                        type: string
                    required:
                    - convertType
                    type: object
                  tolerationConverter:
                    description: TolerationConverter used to modify the pod's Tolerations
                      when pod synced to leaf cluster
                    properties:
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      tolerations:
                        items:
                          description: |-
                            The pod this Toleration is attached to tolerates any taint that matches
                            the triple <key,value,effect> using the matching operator <operator>.
                          properties:
                            effect:
                              description: |-
                                Effect indicates the taint effect to match. Empty means match all taint effects.
                                When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.
                              type: string
                            key:
                              description: |-
                                Key is the taint key that the toleration applies to. Empty means match all taint keys.
                                If the key is empty, operator must be Exists; this combination means to match all values and all keys.
                              type: string
                            operator:
                              description: |-
                                Operator represents a key's relationship to the value.
                                Valid operators are Exists and Equal. Defaults to Equal.
                                Exists is equivalent to wildcard for value, so that a pod can
                                tolerate all taints of a particular category.
                              type: string
                            tolerationSeconds:
                              description: |-
                                TolerationSeconds represents the period of time the toleration (which must be
                                of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default,
                                it is not set, which means tolerate the taint forever (do not evict). Zero and
                                negative values will be treated as 0 (evict immediately) by the system.
                              format: int64
                              type: integer
                            value:
                              description: |-
                                Value is the taint value the toleration matches to.
                                If the operator is Exists, the value should be empty, otherwise just a regular string.
                              type: string
                          type: object
                        type: array
                    required:
                    - convertType
                    type: object
                  topologySpreadConstraintsConverter:
                    description: TopologySpreadConstraintsConverter used to modify
                      the pod's TopologySpreadConstraints when pod synced to leaf
                      cluster
                    properties:
                      convertType:
                        description: ConvertType if the operation type when convert
                          pod from root cluster to leaf cluster.
                        enum:
                        - add
                        - remove
                        - replace
                        type: string
                      topologySpreadConstraints:
                        description: |-
                          TopologySpreadConstraints describes how a group of pods ought to spread across topology
                          domains. Scheduler will schedule pods in a way which abides by the constraints.
                          All topologySpreadConstraints are ANDed.
                        items:
                          description: TopologySpreadConstraint specifies how to spread
                            matching pods among the given topology.
                          properties:
                            labelSelector:
                              description: |-
                                LabelSelector is used to find matching pods.
                                Pods that match this label selector are counted to determine the number of pods
                                in their corresponding topology domain.
                              properties:
                                matchExpressions:
                                  description: matchExpressions is a list of label
                                    selector requirements. The requirements are ANDed.
                                  items:
                                    description: |-
                                      A label selector requirement is a selector that contains values, a key, and an operator that
                                      relates the key and values.
                                    properties:
                                      key:
                                        description: key is the label key that the
                                          selector applies to.
                                        type: string
                                      operator:
                                        description: |-
                                          operator represents a key's relationship to a set of values.
                                          Valid operators are In, NotIn, Exists and DoesNotExist.
                                        type: string
                                      values:
                                        description: |-
                                          values is an array of string values. If the operator is In or NotIn,
                                          the values array must be non-empty. If the operator is Exists or DoesNotExist,
                                          the values array must be empty. This array is replaced during a strategic
                                          merge patch.
                                        items:
                                          type: string
                                        type: array
                                    required:
                                    - key
                                    - operator
                                    type: object
                                  type: array
                                matchLabels:
                                  additionalProperties:
                                    type: string
                                  description: |-
                                    matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                                    map is equivalent to an element of matchExpressions, whose key field is "key", the
                                    operator is "In", and the values array contains only "value". The requirements are ANDed.
                                  type: object
                              type: object
                              x-kubernetes-map-type: atomic
                            matchLabelKeys:
                              description: |-
                                MatchLabelKeys is a set of pod label keys to select the pods over which
                                spreading will be calculated. The keys are used to lookup values from the
                                incoming pod labels, those key-value labels are ANDed with labelSelector
                                to select the group of existing pods over which spreading will be calculated
                                for the incoming pod. Keys that don't exist in the incoming pod labels will
                                be ignored. A null or empty list means only match against labelSelector.
                              items:
                                type: string
                              type: array
                              x-kubernetes-list-type: atomic
                            maxSkew:
                              description: |-
                                MaxSkew describes the degree to which pods may be unevenly distributed.
                                When ` + "`" + `whenUnsatisfiable=DoNotSchedule` + "`" + `, it is the maximum permitted difference
                                between the number of matching pods in the target topology and the global minimum.
                                The global minimum is the minimum number of matching pods in an eligible domain
                                or zero if the number of eligible domains is less than MinDomains.
                                For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same
                                labelSelector spread as 2/2/1:
                                In this case, the global minimum is 1.
                                | zone1 | zone2 | zone3 |
                                |  P P  |  P P  |   P   |
                                - if MaxSkew is 1, incoming pod can only be scheduled to zone3 to become 2/2/2;
                                scheduling it onto zone1(zone2) would make the ActualSkew(3-1) on zone1(zone2)
                                violate MaxSkew(1).
                                - if MaxSkew is 2, incoming pod can be scheduled onto any zone.
                                When ` + "`" + `whenUnsatisfiable=ScheduleAnyway` + "`" + `, it is used to give higher precedence
                                to topologies that satisfy it.
                                It's a required field. Default value is 1 and 0 is not allowed.
                              format: int32
                              type: integer
                            minDomains:
                              description: |-
                                MinDomains indicates a minimum number of eligible domains.
                                When the number of eligible domains with matching topology keys is less than minDomains,
                                Pod Topology Spread treats "global minimum" as 0, and then the calculation of Skew is performed.
                                And when the number of eligible domains with matching topology keys equals or greater than minDomains,
                                this value has no effect on scheduling.
                                As a result, when the number of eligible domains is less than minDomains,
                                scheduler won't schedule more than maxSkew Pods to those domains.
                                If value is nil, the constraint behaves as if MinDomains is equal to 1.
                                Valid values are integers greater than 0.
                                When value is not nil, WhenUnsatisfiable must be DoNotSchedule.


                                For example, in a 3-zone cluster, MaxSkew is set to 2, MinDomains is set to 5 and pods with the same
                                labelSelector spread as 2/2/2:
                                | zone1 | zone2 | zone3 |
                                |  P P  |  P P  |  P P  |
                                The number of domains is less than 5(MinDomains), so "global minimum" is treated as 0.
                                In this situation, new pod with the same labelSelector cannot be scheduled,
                                because computed skew will be 3(3 - 0) if new Pod is scheduled to any of the three zones,
                                it will violate MaxSkew.


                                This is a beta field and requires the MinDomainsInPodTopologySpread feature gate to be enabled (enabled by default).
                              format: int32
                              type: integer
                            nodeAffinityPolicy:
                              description: |-
                                NodeAffinityPolicy indicates how we will treat Pod's nodeAffinity/nodeSelector
                                when calculating pod topology spread skew. Options are:
                                - Honor: only nodes matching nodeAffinity/nodeSelector are included in the calculations.
                                - Ignore: nodeAffinity/nodeSelector are ignored. All nodes are included in the calculations.


                                If this value is nil, the behavior is equivalent to the Honor policy.
                                This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.
                              type: string
                            nodeTaintsPolicy:
                              description: |-
                                NodeTaintsPolicy indicates how we will treat node taints when calculating
                                pod topology spread skew. Options are:
                                - Honor: nodes without taints, along with tainted nodes for which the incoming pod
                                has a toleration, are included.
                                - Ignore: node taints are ignored. All nodes are included.


                                If this value is nil, the behavior is equivalent to the Ignore policy.
                                This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.
                              type: string
                            topologyKey:
                              description: |-
                                TopologyKey is the key of node labels. Nodes that have a label with this key
                                and identical values are considered to be in the same topology.
                                We consider each <key, value> as a "bucket", and try to put balanced number
                                of pods into each bucket.
                                We define a domain as a particular instance of a topology.
                                Also, we define an eligible domain as a domain whose nodes meet the requirements of
                                nodeAffinityPolicy and nodeTaintsPolicy.
                                e.g. If TopologyKey is "kubernetes.io/hostname", each Node is a domain of that topology.
                                And, if TopologyKey is "topology.kubernetes.io/zone", each zone is a domain of that topology.
                                It's a required field.
                              type: string
                            whenUnsatisfiable:
                              description: |-
                                WhenUnsatisfiable indicates how to deal with a pod if it doesn't satisfy
                                the spread constraint.
                                - DoNotSchedule (default) tells the scheduler not to schedule it.
                                - ScheduleAnyway tells the scheduler to schedule the pod in any location,
                                  but giving higher precedence to topologies that would help reduce the
                                  skew.
                                A constraint is considered "Unsatisfiable" for an incoming pod
                                if and only if every possible node assignment for that pod would violate
                                "MaxSkew" on some topology.
                                For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same
                                labelSelector spread as 3/1/1:
                                | zone1 | zone2 | zone3 |
                                | P P P |   P   |   P   |
                                If WhenUnsatisfiable is set to DoNotSchedule, incoming pod can only be scheduled
                                to zone2(zone3) to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3) satisfies
                                MaxSkew(1). In other words, the cluster can still be imbalanced, but scheduler
                                won't make it *more* imbalanced.
                                It's a required field.
                              type: string
                          required:
                          - maxSkew
                          - topologyKey
                          - whenUnsatisfiable
                          type: object
                        type: array
                    required:
                    - convertType
                    type: object
                type: object
              labelSelector:
                description: |-
                  A label query over a set of resources.
                  If name is not empty, labelSelector will be ignored.
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: |-
                        A label selector requirement is a selector that contains values, a key, and an operator that
                        relates the key and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: |-
                            operator represents a key's relationship to a set of values.
                            Valid operators are In, NotIn, Exists and DoesNotExist.
                          type: string
                        values:
                          description: |-
                            values is an array of string values. If the operator is In or NotIn,
                            the values array must be non-empty. If the operator is Exists or DoesNotExist,
                            the values array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: |-
                      matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                      map is equivalent to an element of matchExpressions, whose key field is "key", the
                      operator is "In", and the values array contains only "value". The requirements are ANDed.
                    type: object
                type: object
                x-kubernetes-map-type: atomic
              leafNodeSelector:
                description: |-
                  A label query over a set of resources.
                  If name is not empty, LeafNodeSelector will be ignored.
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: |-
                        A label selector requirement is a selector that contains values, a key, and an operator that
                        relates the key and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: |-
                            operator represents a key's relationship to a set of values.
                            Valid operators are In, NotIn, Exists and DoesNotExist.
                          type: string
                        values:
                          description: |-
                            values is an array of string values. If the operator is In or NotIn,
                            the values array must be non-empty. If the operator is Exists or DoesNotExist,
                            the values array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: |-
                      matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
                      map is equivalent to an element of matchExpressions, whose key field is "key", the
                      operator is "In", and the values array contains only "value". The requirements are ANDed.
                    type: object
                type: object
                x-kubernetes-map-type: atomic
            required:
            - labelSelector
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
