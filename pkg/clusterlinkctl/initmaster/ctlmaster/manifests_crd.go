package ctlmaster

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/clusterlink/pkg/operator/addons/utils"
)

const (
	ClusterNode = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusternodes.clusterlink.io
spec:
  group: clusterlink.io
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

	Cluster = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: clusters.clusterlink.io
spec:
  group: clusterlink.io
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
                default: clusterlink-system
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

	NodeConfig = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.0
  creationTimestamp: null
  name: nodeconfigs.clusterlink.io
spec:
  group: clusterlink.io
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
)

type CRDReplace struct {
}

func MakeCRD(crdTemplate string) func() (*apiextensionsv1.CustomResourceDefinition, error) {
	return func() (*apiextensionsv1.CustomResourceDefinition, error) {
		klog.Info("init Clusterlink CRD")
		clusterlinkBytes, err := utils.ParseTemplate(crdTemplate,
			CRDReplace{})

		if err != nil {
			return nil, fmt.Errorf("error when parsing clusterlink CRD clusternode template :%v", err)
		} else if clusterlinkBytes == nil {
			return nil, fmt.Errorf("crd clusterlink template get nil")
		}

		crdStruct := &apiextensionsv1.CustomResourceDefinition{}

		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(),
			clusterlinkBytes, crdStruct); err != nil {
			return nil, fmt.Errorf("decode clusterlink ClusterRoleBindingBytes error : %v ", err)
		}
		return crdStruct, nil
	}

}
