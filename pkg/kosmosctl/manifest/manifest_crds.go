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

const ClusterNode = `---
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

const NodeConfig = `---
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

type CRDReplace struct {
	Namespace string
}
