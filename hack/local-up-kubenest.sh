#!/usr/bin/env bash

REPO_ROOT=$(git rev-parse --show-toplevel)

GLOBALNODES_YAML_PATH=${REPO_ROOT}/deploy/crds/kosmos.io_globalnodes.yaml
CRDS_YAML_PATH=${REPO_ROOT}/deploy/crds/kosmos.io_virtualclusters.yaml
GLOBALNODES_HELPER_PATH=${REPO_ROOT}/hack/k8s-in-k8s/globalnodes_helper.sh
IMAGE_REPOSITIRY={}
DOMAIN_PORT=$(echo $IMAGE_REPOSITIRY | cut -d'/' -f1)
CONFIG_FILE=${REPO_ROOT}/hack/kind-k8s-in-k8s-config.yaml

# 检查 IMAGE_REPOSITIRY 是否为空，并检测镜像是否齐全
check_images_in_repository() {
  local repository="$1"
  local images=("${!2}")

  # 检查 IMAGE_REPOSITIRY 是否为空
  if [ -z "$repository" ]; then
    echo "错误：IMAGE_REPOSITIRY 未配置，请先配置镜像仓库地址。"
    exit 1 # 停止脚本执行，返回错误码 1
  fi

  # 记录缺失的镜像
  local missing_images=()

  # 检查每个镜像是否存在于仓库中
  for IMAGE in "${images[@]}"; do
    # 完整的镜像路径
    local full_image="$repository/$IMAGE"

    # 使用 docker manifest inspect 来检查镜像的元数据
    if ! docker pull --quiet $full_image >/dev/null 2>&1; then
      echo "缺少镜像: $full_image"
      missing_images+=("$full_image")
    else
      echo "镜像存在: $full_image"
    fi
  done

  # 如果有缺失的镜像，则停止脚本执行
  if [ ${#missing_images[@]} -ne 0 ]; then
    echo "错误：以下镜像在仓库中不存在："
    for missing in "${missing_images[@]}"; do
      echo "- $missing"
    done
    exit 1
  fi

  echo "所有镜像都已存在。"
}

# 定义需要检测的镜像列表
IMAGES=(
  "kindest-node:v1.25.8"
  "openebs/node-disk-manager:2.0.0"
  "openebs/node-disk-operator:2.0.0"
  "openebs/linux-utils:3.3.0"
  "openebs/node-disk-exporter:2.0.0"
  "openebs/provisioner-localpv:3.3.0"
  "virtual-cluster-operator:latest"
  "node-agent:latest"
  "kas-network-proxy-server:v1.25.7-eki.3.0.0"
  "kas-network-proxy-agent:v1.25.7-eki.3.0.0"
  "kube-apiserver:v1.25.7-eki.3.0.0"
  "scheduler:v1.25.7-eki.3.0.0"
  "kube-proxy:v1.25.7-eki.3.0.0"
  "kube-controller-manager:v1.25.7-eki.3.0.0"
  "etcd:v1.25.7-eki.3.0.0"
  "keepalived:v1.25.7-eki.3.0.0"
  "kubectl:v1.25.7-eki.3.0.0"
)

# 调用函数进行镜像检查
check_images_in_repository "$IMAGE_REPOSITIRY" IMAGES[@]

function generate_kind_config() {
  local config_file=$1
  cat <<EOF >$1
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: k8s-in-k8s
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${DOMAIN_PORT}"]
    endpoint = ["http://${DOMAIN_PORT}"]
nodes:
  - role: control-plane
    image: ${IMAGE_REPOSITIRY}/kindest-node:v1.25.8
    extraMounts:
    - hostPath: ~/root
      containerPath: /apps
    - hostPath: /run/udev
      containerPath: /run/udev
$(for i in {1..4}; do
    cat <<END
  - role: worker
    image: ${IMAGE_REPOSITIRY}/kindest-node:v1.25.8
    extraMounts:
    - hostPath: ~/root
      containerPath: /apps
    - hostPath: /run/udev
      containerPath: /run/udev
END
  done)
EOF
}

check_cluster_exists() {
  kind get clusters | grep -q "^kubenest$"
  return $?
}

all_nodes_ready() {
  local ready_nodes
  ready_nodes=$(kubectl get nodes --no-headers | grep -c " Ready")
  if [ "$ready_nodes" -eq "$TOTAL_NODES" ]; then
    return 0 # 所有节点Ready
  else
    return 1 # 还有节点未Ready
  fi
}

# 创建并等待Kind集群准备好
create_and_wait_for_cluster() {
  kind create cluster -n kubenest --config $CONFIG_FILE
  TOTAL_NODES=$(kubectl get nodes --no-headers | wc -l)

  # 循环等待，直到所有节点都Ready
  while ! all_nodes_ready; do
    echo "等待所有节点进入Ready状态..."
    sleep 5 # 等待5秒再检查一次
  done
  echo "所有节点已准备好。"
}
generate_kind_config "kind-k8s-in-k8s-config.yaml"
if check_cluster_exists; then
  echo "kubenest集群已存在，继续执行。"
else
  create_and_wait_for_cluster
fi

#helm repo add openebs https://openebs.github.io/openebs
#helm repo update
#helm install openebs --namespace openebs openebs/openebs --set engines.replicated.mayastor.enabled=false --create-namespace
#kubectl get po -n openebs
#通过yaml的方式安装openebs

cat <<EOF >openebs-hostpath.yaml
# This manifest deploys the OpenEBS control plane components,
# with associated CRs & RBAC rules
# NOTE: On GKE, deploy the openebs-operator.yaml in admin context
#
# NOTE: The Jiva and cStor components previously included in the Operator File
#  are now removed and it is recommended for users to use cStor and Jiva CSI operators.
#  To upgrade your Jiva and cStor volumes to CSI, please checkout the documentation at:
#  https://github.com/openebs/upgrade
#
# To deploy the legacy Jiva and cStor:
# kubectl apply -f https://openebs.github.io/charts/legacy-openebs-operator.yaml
#
# To deploy cStor CSI:
# kubectl apply -f https://openebs.github.io/charts/cstor-operator.yaml
#
# To deploy Jiva CSI:
# kubectl apply -f https://openebs.github.io/charts/jiva-operator.yaml
#

# Create the OpenEBS namespace
apiVersion: v1
kind: Namespace
metadata:
  name: openebs
---
# Create Maya Service Account
apiVersion: v1
kind: ServiceAccount
metadata:
  name: openebs-maya-operator
  namespace: openebs
---
# Define Role that allows operations on K8s pods/deployments
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: openebs-maya-operator
rules:
  - apiGroups: ["*"]
    resources: ["nodes", "nodes/proxy"]
    verbs: ["*"]
  - apiGroups: ["*"]
    resources: ["namespaces", "services", "pods", "pods/exec", "deployments", "deployments/finalizers", "replicationcontrollers", "replicasets", "events", "endpoints", "configmaps", "secrets", "jobs", "cronjobs"]
    verbs: ["*"]
  - apiGroups: ["*"]
    resources: ["statefulsets", "daemonsets"]
    verbs: ["*"]
  - apiGroups: ["*"]
    resources: ["resourcequotas", "limitranges"]
    verbs: ["list", "watch"]
  - apiGroups: ["*"]
    resources: ["ingresses", "horizontalpodautoscalers", "verticalpodautoscalers", "certificatesigningrequests"]
    verbs: ["list", "watch"]
  - apiGroups: ["*"]
    resources: ["storageclasses", "persistentvolumeclaims", "persistentvolumes"]
    verbs: ["*"]
  - apiGroups: ["volumesnapshot.external-storage.k8s.io"]
    resources: ["volumesnapshots", "volumesnapshotdatas"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apiextensions.k8s.io"]
    resources: ["customresourcedefinitions"]
    verbs: [ "get", "list", "create", "update", "delete", "patch"]
  - apiGroups: ["openebs.io"]
    resources: [ "*"]
    verbs: ["*" ]
  - apiGroups: ["cstor.openebs.io"]
    resources: [ "*"]
    verbs: ["*" ]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "watch", "list", "delete", "update", "create"]
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
    verbs: ["get", "create", "list", "delete", "update", "patch"]
  - nonResourceURLs: ["/metrics"]
    verbs: ["get"]
  - apiGroups: ["*"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "create", "delete", "watch"]
---
# Bind the Service Account with the Role Privileges.
# TODO: Check if default account also needs to be there
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: openebs-maya-operator
subjects:
  - kind: ServiceAccount
    name: openebs-maya-operator
    namespace: openebs
roleRef:
  kind: ClusterRole
  name: openebs-maya-operator
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.5.0
  creationTimestamp: null
  name: blockdevices.openebs.io
spec:
  group: openebs.io
  names:
    kind: BlockDevice
    listKind: BlockDeviceList
    plural: blockdevices
    shortNames:
      - bd
    singular: blockdevice
  scope: Namespaced
  versions:
    - additionalPrinterColumns:
        - jsonPath: .spec.nodeAttributes.nodeName
          name: NodeName
          type: string
        - jsonPath: .spec.path
          name: Path
          priority: 1
          type: string
        - jsonPath: .spec.filesystem.fsType
          name: FSType
          priority: 1
          type: string
        - jsonPath: .spec.capacity.storage
          name: Size
          type: string
        - jsonPath: .status.claimState
          name: ClaimState
          type: string
        - jsonPath: .status.state
          name: Status
          type: string
        - jsonPath: .metadata.creationTimestamp
          name: Age
          type: date
      name: v1alpha1
      schema:
        openAPIV3Schema:
          description: BlockDevice is the Schema for the blockdevices API
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
              type: string
            metadata:
              type: object
            spec:
              description: DeviceSpec defines the properties and runtime status of a BlockDevice
              properties:
                aggregateDevice:
                  description: AggregateDevice was intended to store the hierarchical information in cases of LVM. However this is currently not implemented and may need to be re-looked into for better design. To be deprecated
                  type: string
                capacity:
                  description: Capacity
                  properties:
                    logicalSectorSize:
                      description: LogicalSectorSize is blockdevice logical-sector size in bytes
                      format: int32
                      type: integer
                    physicalSectorSize:
                      description: PhysicalSectorSize is blockdevice physical-Sector size in bytes
                      format: int32
                      type: integer
                    storage:
                      description: Storage is the blockdevice capacity in bytes
                      format: int64
                      type: integer
                  required:
                    - storage
                  type: object
                claimRef:
                  description: ClaimRef is the reference to the BDC which has claimed this BD
                  properties:
                    apiVersion:
                      description: API version of the referent.
                      type: string
                    fieldPath:
                      description: 'If referring to a piece of an object instead of an entire object, this string should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2]. For example, if the object reference is to a container within a pod, this would take on a value like: "spec.containers{name}" (where "name" refers to the name of the container that triggered the event) or if no container name is specified "spec.containers[2]" (container with index 2 in this pod). This syntax is chosen only to have some well-defined way of referencing a part of an object. TODO: this design is not final and this field is subject to change in the future.'
                      type: string
                    kind:
                      description: 'Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
                      type: string
                    name:
                      description: 'Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names'
                      type: string
                    namespace:
                      description: 'Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/'
                      type: string
                    resourceVersion:
                      description: 'Specific resourceVersion to which this reference is made, if any. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency'
                      type: string
                    uid:
                      description: 'UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids'
                      type: string
                  type: object
                details:
                  description: Details contain static attributes of BD like model,serial, and so forth
                  properties:
                    compliance:
                      description: Compliance is standards/specifications version implemented by device firmware  such as SPC-1, SPC-2, etc
                      type: string
                    deviceType:
                      description: DeviceType represents the type of device like sparse, disk, partition, lvm, crypt
                      enum:
                        - disk
                        - partition
                        - sparse
                        - loop
                        - lvm
                        - crypt
                        - dm
                        - mpath
                      type: string
                    driveType:
                      description: DriveType is the type of backing drive, HDD/SSD
                      enum:
                        - HDD
                        - SSD
                        - Unknown
                        - ""
                      type: string
                    firmwareRevision:
                      description: FirmwareRevision is the disk firmware revision
                      type: string
                    hardwareSectorSize:
                      description: HardwareSectorSize is the hardware sector size in bytes
                      format: int32
                      type: integer
                    logicalBlockSize:
                      description: LogicalBlockSize is the logical block size in bytes reported by /sys/class/block/sda/queue/logical_block_size
                      format: int32
                      type: integer
                    model:
                      description: Model is model of disk
                      type: string
                    physicalBlockSize:
                      description: PhysicalBlockSize is the physical block size in bytes reported by /sys/class/block/sda/queue/physical_block_size
                      format: int32
                      type: integer
                    serial:
                      description: Serial is serial number of disk
                      type: string
                    vendor:
                      description: Vendor is vendor of disk
                      type: string
                  type: object
                devlinks:
                  description: DevLinks contains soft links of a block device like /dev/by-id/... /dev/by-uuid/...
                  items:
                    description: DeviceDevLink holds the mapping between type and links like by-id type or by-path type link
                    properties:
                      kind:
                        description: Kind is the type of link like by-id or by-path.
                        enum:
                          - by-id
                          - by-path
                        type: string
                      links:
                        description: Links are the soft links
                        items:
                          type: string
                        type: array
                    type: object
                  type: array
                filesystem:
                  description: FileSystem contains mountpoint and filesystem type
                  properties:
                    fsType:
                      description: Type represents the FileSystem type of the block device
                      type: string
                    mountPoint:
                      description: MountPoint represents the mountpoint of the block device.
                      type: string
                  type: object
                nodeAttributes:
                  description: NodeAttributes has the details of the node on which BD is attached
                  properties:
                    nodeName:
                      description: NodeName is the name of the Kubernetes node resource on which the device is attached
                      type: string
                  type: object
                parentDevice:
                  description: "ParentDevice was intended to store the UUID of the parent Block Device as is the case for partitioned block devices. \n For example: /dev/sda is the parent for /dev/sda1 To be deprecated"
                  type: string
                partitioned:
                  description: Partitioned represents if BlockDevice has partitions or not (Yes/No) Currently always default to No. To be deprecated
                  enum:
                    - "Yes"
                    - "No"
                  type: string
                path:
                  description: Path contain devpath (e.g. /dev/sdb)
                  type: string
              required:
                - capacity
                - devlinks
                - nodeAttributes
                - path
              type: object
            status:
              description: DeviceStatus defines the observed state of BlockDevice
              properties:
                claimState:
                  description: ClaimState represents the claim state of the block device
                  enum:
                    - Claimed
                    - Unclaimed
                    - Released
                  type: string
                state:
                  description: State is the current state of the blockdevice (Active/Inactive/Unknown)
                  enum:
                    - Active
                    - Inactive
                    - Unknown
                  type: string
              required:
                - claimState
                - state
              type: object
          type: object
      served: true
      storage: true
      subresources: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []

---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.5.0
  creationTimestamp: null
  name: blockdeviceclaims.openebs.io
spec:
  group: openebs.io
  names:
    kind: BlockDeviceClaim
    listKind: BlockDeviceClaimList
    plural: blockdeviceclaims
    shortNames:
      - bdc
    singular: blockdeviceclaim
  scope: Namespaced
  versions:
    - additionalPrinterColumns:
        - jsonPath: .spec.blockDeviceName
          name: BlockDeviceName
          type: string
        - jsonPath: .status.phase
          name: Phase
          type: string
        - jsonPath: .metadata.creationTimestamp
          name: Age
          type: date
      name: v1alpha1
      schema:
        openAPIV3Schema:
          description: BlockDeviceClaim is the Schema for the blockdeviceclaims API
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
              type: string
            metadata:
              type: object
            spec:
              description: DeviceClaimSpec defines the request details for a BlockDevice
              properties:
                blockDeviceName:
                  description: BlockDeviceName is the reference to the block-device backing this claim
                  type: string
                blockDeviceNodeAttributes:
                  description: BlockDeviceNodeAttributes is the attributes on the node from which a BD should be selected for this claim. It can include nodename, failure domain etc.
                  properties:
                    hostName:
                      description: HostName represents the hostname of the Kubernetes node resource where the BD should be present
                      type: string
                    nodeName:
                      description: NodeName represents the name of the Kubernetes node resource where the BD should be present
                      type: string
                  type: object
                deviceClaimDetails:
                  description: Details of the device to be claimed
                  properties:
                    allowPartition:
                      description: AllowPartition represents whether to claim a full block device or a device that is a partition
                      type: boolean
                    blockVolumeMode:
                      description: 'BlockVolumeMode represents whether to claim a device in Block mode or Filesystem mode. These are use cases of BlockVolumeMode: 1) Not specified: VolumeMode check will not be effective 2) VolumeModeBlock: BD should not have any filesystem or mountpoint 3) VolumeModeFileSystem: BD should have a filesystem and mountpoint. If DeviceFormat is    specified then the format should match with the FSType in BD'
                      type: string
                    formatType:
                      description: Format of the device required, eg:ext4, xfs
                      type: string
                  type: object
                deviceType:
                  description: DeviceType represents the type of drive like SSD, HDD etc.,
                  nullable: true
                  type: string
                hostName:
                  description: Node name from where blockdevice has to be claimed. To be deprecated. Use NodeAttributes.HostName instead
                  type: string
                resources:
                  description: Resources will help with placing claims on Capacity, IOPS
                  properties:
                    requests:
                      additionalProperties:
                        anyOf:
                          - type: integer
                          - type: string
                        pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                        x-kubernetes-int-or-string: true
                      description: 'Requests describes the minimum resources required. eg: if storage resource of 10G is requested minimum capacity of 10G should be available TODO for validating'
                      type: object
                  required:
                    - requests
                  type: object
                selector:
                  description: Selector is used to find block devices to be considered for claiming
                  properties:
                    matchExpressions:
                      description: matchExpressions is a list of label selector requirements. The requirements are ANDed.
                      items:
                        description: A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.
                        properties:
                          key:
                            description: key is the label key that the selector applies to.
                            type: string
                          operator:
                            description: operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
                            type: string
                          values:
                            description: values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.
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
                      description: matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.
                      type: object
                  type: object
              type: object
            status:
              description: DeviceClaimStatus defines the observed state of BlockDeviceClaim
              properties:
                phase:
                  description: Phase represents the current phase of the claim
                  type: string
              required:
                - phase
              type: object
          type: object
      served: true
      storage: true
      subresources: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
---
# This is the node-disk-manager related config.
# It can be used to customize the disks probes and filters
apiVersion: v1
kind: ConfigMap
metadata:
  name: openebs-ndm-config
  namespace: openebs
  labels:
    openebs.io/component-name: ndm-config
data:
  # udev-probe is default or primary probe it should be enabled to run ndm
  # filterconfigs contains configs of filters. To provide a group of include
  # and exclude values add it as , separated string
  node-disk-manager.config: |
    probeconfigs:
      - key: udev-probe
        name: udev probe
        state: true
      - key: seachest-probe
        name: seachest probe
        state: false
      - key: smart-probe
        name: smart probe
        state: true
    filterconfigs:
      - key: os-disk-exclude-filter
        name: os disk exclude filter
        state: true
        exclude: "/,/etc/hosts,/boot"
      - key: vendor-filter
        name: vendor filter
        state: true
        include: ""
        exclude: "CLOUDBYT,OpenEBS"
      - key: path-filter
        name: path filter
        state: true
        include: ""
        exclude: "/dev/loop,/dev/fd0,/dev/sr0,/dev/ram,/dev/md,/dev/dm-,/dev/rbd,/dev/zd"
    # metconfig can be used to decorate the block device with different types of labels
    # that are available on the node or come in a device properties.
    # node labels - the node where bd is discovered. A whitlisted label prefixes
    # attribute labels - a property of the BD can be added as a ndm label as ndm.io/<property>=<property-value>
    metaconfigs:
      - key: node-labels
        name: node labels
        pattern: ""
      - key: device-labels
        name: device labels
        type: ""
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: openebs-ndm
  namespace: openebs
  labels:
    name: openebs-ndm
    openebs.io/component-name: ndm
    openebs.io/version: 3.4.0
spec:
  selector:
    matchLabels:
      name: openebs-ndm
      openebs.io/component-name: ndm
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        name: openebs-ndm
        openebs.io/component-name: ndm
        openebs.io/version: 3.4.0
    spec:
      # By default the node-disk-manager will be run on all kubernetes nodes
      # If you would like to limit this to only some nodes, say the nodes
      # that have storage attached, you could label those node and use
      # nodeSelector.
      #
      # e.g. label the storage nodes with - "openebs.io/nodegroup"="storage-node"
      # kubectl label node <node-name> "openebs.io/nodegroup"="storage-node"
      #nodeSelector:
      #  "openebs.io/nodegroup": "storage-node"
      serviceAccountName: openebs-maya-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      hostNetwork: true
      nodeSelector:
        #beta.kubernetes.io/arch: amd64
        node-role.kubernetes.io/control-plane: ""
      # host PID is used to check status of iSCSI Service when the NDM
      # API service is enabled
      #hostPID: true
      containers:
        - name: node-disk-manager
          image: ${IMAGE_REPOSITIRY}/openebs/node-disk-manager:2.0.0
          args:
            - -v=4
            # The feature-gate is used to enable the new UUID algorithm.
            - --feature-gates="GPTBasedUUID"
          # Use partition table UUID instead of create single partition to get
          # partition UUID. Require  to be enabled with.
          # - --feature-gates="PartitionTableUUID"
          # Detect changes to device size, filesystem and mount-points without restart.
          # - --feature-gates="ChangeDetection"
          # The feature gate is used to start the gRPC API service. The gRPC server
          # starts at 9115 port by default. This feature is currently in Alpha state
          # - --feature-gates="APIService"
          # The feature gate is used to enable NDM, to create blockdevice resources
          # for unused partitions on the OS disk
          # - --feature-gates="UseOSDisk"
          imagePullPolicy: IfNotPresent
          securityContext:
            privileged: true
          volumeMounts:
            - name: config
              mountPath: /host/node-disk-manager.config
              subPath: node-disk-manager.config
              readOnly: true
              # make udev database available inside container
            - name: udev
              mountPath: /run/udev
            - name: procmount
              mountPath: /host/proc
              readOnly: true
            - name: devmount
              mountPath: /dev
            - name: basepath
              mountPath: /var/openebs/ndm
            - name: sparsepath
              mountPath: /var/openebs/sparse
          env:
            # namespace in which NDM is installed will be passed to NDM Daemonset
            # as environment variable
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            # pass hostname as env variable using downward API to the NDM container
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            # specify the directory where the sparse files need to be created.
            # if not specified, then sparse files will not be created.
            - name: SPARSE_FILE_DIR
              value: "/var/openebs/sparse"
            # Size(bytes) of the sparse file to be created.
            - name: SPARSE_FILE_SIZE
              value: "10737418240"
            # Specify the number of sparse files to be created
            - name: SPARSE_FILE_COUNT
              value: "0"
          livenessProbe:
            exec:
              command:
                - pgrep
                - "ndm"
            initialDelaySeconds: 30
            periodSeconds: 60
      volumes:
        - name: config
          configMap:
            name: openebs-ndm-config
        - name: udev
          hostPath:
            path: /run/udev
            type: Directory
        # mount /proc (to access mount file of process 1 of host) inside container
        # to read mount-point of disks and partitions
        - name: procmount
          hostPath:
            path: /proc
            type: Directory
        - name: devmount
          # the /dev directory is mounted so that we have access to the devices that
          # are connected at runtime of the pod.
          hostPath:
            path: /dev
            type: Directory
        - name: basepath
          hostPath:
            path: /var/openebs/ndm
            type: DirectoryOrCreate
        - name: sparsepath
          hostPath:
            path: /var/openebs/sparse
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openebs-ndm-operator
  namespace: openebs
  labels:
    name: openebs-ndm-operator
    openebs.io/component-name: ndm-operator
    openebs.io/version: 3.4.0
spec:
  selector:
    matchLabels:
      name: openebs-ndm-operator
      openebs.io/component-name: ndm-operator
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        name: openebs-ndm-operator
        openebs.io/component-name: ndm-operator
        openebs.io/version: 3.4.0
    spec:
      serviceAccountName: openebs-maya-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        #beta.kubernetes.io/arch: amd64
        node-role.kubernetes.io/control-plane: ""
      containers:
        - name: node-disk-operator
          image: ${IMAGE_REPOSITIRY}/openebs/node-disk-operator:2.0.0
          imagePullPolicy: IfNotPresent
          env:
            - name: WATCH_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            # the service account of the ndm-operator pod
            - name: SERVICE_ACCOUNT
              valueFrom:
                fieldRef:
                  fieldPath: spec.serviceAccountName
            - name: OPERATOR_NAME
              value: "node-disk-operator"
            - name: CLEANUP_JOB_IMAGE
              value: "${IMAGE_REPOSITIRY}/openebs/linux-utils:3.3.0"
            # OPENEBS_IO_IMAGE_PULL_SECRETS environment variable is used to pass the image pull secrets
            # to the cleanup pod launched by NDM operator
            #- name: OPENEBS_IO_IMAGE_PULL_SECRETS
            #  value: ""
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8585
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8585
            initialDelaySeconds: 5
            periodSeconds: 10
---
# Create NDM cluster exporter deployment.
# This is an optional component and is not required for the basic
# functioning of NDM
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openebs-ndm-cluster-exporter
  namespace: openebs
  labels:
    name: openebs-ndm-cluster-exporter
    openebs.io/component-name: ndm-cluster-exporter
    openebs.io/version: 3.4.0
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      name: openebs-ndm-cluster-exporter
      openebs.io/component-name: ndm-cluster-exporter
  template:
    metadata:
      labels:
        name: openebs-ndm-cluster-exporter
        openebs.io/component-name: ndm-cluster-exporter
        openebs.io/version: 3.4.0
    spec:
      serviceAccountName: openebs-maya-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        #beta.kubernetes.io/arch: amd64
        node-role.kubernetes.io/control-plane: ""
      containers:
        - name: ndm-cluster-exporter
          image: ${IMAGE_REPOSITIRY}/openebs/node-disk-exporter:2.0.0
          command:
            - /usr/local/bin/exporter
          args:
            - "start"
            - "--mode=cluster"
            - "--port=\$(METRICS_LISTEN_PORT)"
            - "--metrics=/metrics"
          ports:
            - containerPort: 9100
              protocol: TCP
              name: metrics
          imagePullPolicy: IfNotPresent
          env:
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: METRICS_LISTEN_PORT
              value: :9100
---
# Create NDM cluster exporter service
# This is optional and required only when
# ndm-cluster-exporter deployment is used
apiVersion: v1
kind: Service
metadata:
  name: openebs-ndm-cluster-exporter-service
  namespace: openebs
  labels:
    name: openebs-ndm-cluster-exporter-service
    openebs.io/component-name: ndm-cluster-exporter
    app: openebs-ndm-exporter
spec:
  clusterIP: None
  ports:
    - name: metrics
      port: 9100
      targetPort: 9100
  selector:
    name: openebs-ndm-cluster-exporter
---
# Create NDM node exporter daemonset.
# This is an optional component used for getting disk level
# metrics from each of the storage nodes
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: openebs-ndm-node-exporter
  namespace: openebs
  labels:
    name: openebs-ndm-node-exporter
    openebs.io/component-name: ndm-node-exporter
    openebs.io/version: 3.4.0
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      name: openebs-ndm-node-exporter
      openebs.io/component-name: ndm-node-exporter
  template:
    metadata:
      labels:
        name: openebs-ndm-node-exporter
        openebs.io/component-name: ndm-node-exporter
        openebs.io/version: 3.4.0
    spec:
      serviceAccountName: openebs-maya-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        #beta.kubernetes.io/arch: amd64
        node-role.kubernetes.io/control-plane: ""
      containers:
        - name: node-disk-exporter
          image: ${IMAGE_REPOSITIRY}/openebs/node-disk-exporter:2.0.0
          command:
            - /usr/local/bin/exporter
          args:
            - "start"
            - "--mode=node"
            - "--port=\$(METRICS_LISTEN_PORT)"
            - "--metrics=/metrics"
          ports:
            - containerPort: 9101
              protocol: TCP
              name: metrics
          imagePullPolicy: IfNotPresent
          securityContext:
            privileged: true
          env:
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: METRICS_LISTEN_PORT
              value: :9101
---
# Create NDM node exporter service
# This is optional and required only when
# ndm-node-exporter daemonset is used
apiVersion: v1
kind: Service
metadata:
  name: openebs-ndm-node-exporter-service
  namespace: openebs
  labels:
    name: openebs-ndm-node-exporter
    openebs.io/component: openebs-ndm-node-exporter
    app: openebs-ndm-exporter
spec:
  clusterIP: None
  ports:
    - name: metrics
      port: 9101
      targetPort: 9101
  selector:
    name: openebs-ndm-node-exporter
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openebs-localpv-provisioner
  namespace: openebs
  labels:
    name: openebs-localpv-provisioner
    openebs.io/component-name: openebs-localpv-provisioner
    openebs.io/version: 3.4.0
spec:
  selector:
    matchLabels:
      name: openebs-localpv-provisioner
      openebs.io/component-name: openebs-localpv-provisioner
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        name: openebs-localpv-provisioner
        openebs.io/component-name: openebs-localpv-provisioner
        openebs.io/version: 3.4.0
    spec:
      serviceAccountName: openebs-maya-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        #beta.kubernetes.io/arch: amd64
        node-role.kubernetes.io/control-plane: ""
      containers:
        - name: openebs-provisioner-hostpath
          imagePullPolicy: IfNotPresent
          image: ${IMAGE_REPOSITIRY}/openebs/provisioner-localpv:3.3.0
          args:
            - "--bd-time-out=\$(BDC_BD_BIND_RETRIES)"
          env:
            # OPENEBS_IO_K8S_MASTER enables openebs provisioner to connect to K8s
            # based on this address. This is ignored if empty.
            # This is supported for openebs provisioner version 0.5.2 onwards
            #- name: OPENEBS_IO_K8S_MASTER
            #  value: "http://10.128.0.12:8080"
            # OPENEBS_IO_KUBE_CONFIG enables openebs provisioner to connect to K8s
            # based on this config. This is ignored if empty.
            # This is supported for openebs provisioner version 0.5.2 onwards
            #- name: OPENEBS_IO_KUBE_CONFIG
            #  value: "/home/ubuntu/.kube/config"
            # This sets the number of times the provisioner should try
            # with a polling interval of 5 seconds, to get the Blockdevice
            # Name from a BlockDeviceClaim, before the BlockDeviceClaim
            # is deleted. E.g. 12 * 5 seconds = 60 seconds timeout
            - name: BDC_BD_BIND_RETRIES
              value: "12"
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: OPENEBS_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            # OPENEBS_SERVICE_ACCOUNT provides the service account of this pod as
            # environment variable
            - name: OPENEBS_SERVICE_ACCOUNT
              valueFrom:
                fieldRef:
                  fieldPath: spec.serviceAccountName
            - name: OPENEBS_IO_ENABLE_ANALYTICS
              value: "true"
            - name: OPENEBS_IO_INSTALLER_TYPE
              value: "openebs-operator"
            - name: OPENEBS_IO_HELPER_IMAGE
              value: "${IMAGE_REPOSITIRY}/openebs/linux-utils:3.3.0"
            - name: OPENEBS_IO_BASE_PATH
              value: "/var/openebs/local"
          # LEADER_ELECTION_ENABLED is used to enable/disable leader election. By default
          # leader election is enabled.
          #- name: LEADER_ELECTION_ENABLED
          #  value: "true"
          # OPENEBS_IO_IMAGE_PULL_SECRETS environment variable is used to pass the image pull secrets
          # to the helper pod launched by local-pv hostpath provisioner
          #- name: OPENEBS_IO_IMAGE_PULL_SECRETS
          #  value: ""
          # Process name used for matching is limited to the 15 characters
          # present in the pgrep output.
          # So fullname can't be used here with pgrep (>15 chars).A regular expression
          # that matches the entire command name has to specified.
          # Anchor  : matches any string that starts with 
          # : matches any string that has  followed by zero or more char
          livenessProbe:
            exec:
              command:
                - sh
                - -c
                - test \`pgrep -c "^provisioner-loc.*"\` = 1
            initialDelaySeconds: 30
            periodSeconds: 60
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: openebs-hostpath
  annotations:
    openebs.io/cas-type: local
    cas.openebs.io/config: |
      #hostpath type will create a PV by
      # creating a sub-directory under the
      # BASEPATH provided below.
      - name: StorageType
        value: "hostpath"
      #Specify the location (directory) where
      # where PV(volume) data will be saved.
      # A sub-directory with pv-name will be
      # created. When the volume is deleted,
      # the PV sub-directory will be deleted.
      #Default value is /var/openebs/local
      - name: BasePath
        value: "/apps/data/kosmos_etcd"
provisioner: openebs.io/local
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: openebs-device
  annotations:
    openebs.io/cas-type: local
    cas.openebs.io/config: |
      #device type will create a PV by
      # issuing a BDC and will extract the path
      # values from the associated BD.
      - name: StorageType
        value: "device"
provisioner: openebs.io/local
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
---

EOF

kubectl apply -f openebs-hostpath.yaml

# 检查openebs命名空间中的所有Pod是否都处于Running状态
function all_pods_running() {
  local pod_status
  pod_status=$(kubectl get po -n openebs --no-headers | awk '{print $3}' | grep -v "Running")

  if [ -z "$pod_status" ]; then
    return 0 # 所有Pod处于Running状态
  else
    return 1 # 还有Pod未处于Running状态
  fi
}

# 循环等待，直到openebs命名空间中的所有Pod都Running
while ! all_pods_running; do
  echo "等待所有openebs命名空间中的Pod进入Running状态..."
  sleep 5 # 等待5秒再检查一次
done

echo "所有openebs命名空间中的Pod均已Running，继续执行后续操作。"

kubectl config view --flatten=true --minify=true >host-config

# 验证生成的 hostKubeconfig 文件
kubectl get nodes --kubeconfig host-config

#  获取节点信息
kubectl get node -o wide

# 提取一个 kubenest-control-plane 节点的 INTERNAL-IP
echo "Step 8: 提取一个 kubenest-control-plane 节点的 INTERNAL-IP"
control_plane_ip=$(kubectl get nodes -o wide | grep 'kubenest-control-plane' | awk '{print $6}' | head -n 1)
if [ -z "$control_plane_ip" ]; then
  echo "Error: 未找到 kubenest-control-plane 节点的 INTERNAL-IP"
  exit 1
else
  echo "Control Plane Internal IP: $control_plane_ip"
fi

# 修改 host-config 文件，替换 server 字段中的 IP 地址
sed -i "s|server: .*|server: https://$control_plane_ip:6443|g" host-config

cat host-config

echo "Step 11: 生成 host-config 文件的 base64 编码"
host_config_base64=$(cat host-config | base64 -w0)
echo "Base64 encoded host-config:"
echo $host_config_base64

# Step 12: 创建 vc-operator.yaml
cat <<EOF >vc-operator.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: kosmos-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtual-cluster-operator
rules:
  - apiGroups: ['*']
    resources: ['*']
    verbs: ["*"]
  - nonResourceURLs: ['*']
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: virtual-cluster-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: virtual-cluster-operator
subjects:
  - kind: ServiceAccount
    name: virtual-cluster-operator
    namespace: kosmos-system
---
apiVersion: v1
kind: Secret
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
type: Opaque
data:
  kubeconfig: $host_config_base64
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
data:
  env.sh: |
    #!/usr/bin/env bash
    
    # #####
    # Generate by script generate_env.sh
    # #####
    
    SCRIPT_VERSION=0.0.1
    # tmp dir of kosmos
    PATH_FILE_TMP=/apps/conf/kosmos/tmp
    ##################################################
    # path for kubeadm config
    PATH_KUBEADM_CONFIG=/etc/kubeadm
    ##################################################
    # path for kubernetes, from kubelet args --config
    PATH_KUBERNETES=/etc/kubernetes
    PATH_KUBERNETES_PKI=/etc/kubernetes/pki
    # name for kubelet kubeconfig file
    KUBELET_KUBE_CONFIG_NAME=kubelet.conf
    ##################################################
    # path for kubelet
    PATH_KUBELET_LIB=/var/lib/kubelet
    # path for kubelet
    PATH_KUBELET_CONF=/var/lib/kubelet
    # name for config file of kubelet
    KUBELET_CONFIG_NAME=config.yaml
    HOST_CORE_DNS=10.96.0.10
    # kubeadm switch
    USE_KUBEADM=false
    # Generate kubelet.conf TIMEOUT
    KUBELET_CONF_TIMEOUT=30
    
    function GenerateKubeadmConfig() {
        echo "---
    apiVersion: kubeadm.k8s.io/v1beta2
    caCertPath: /etc/kubernetes/pki/ca.crt
    discovery:
        bootstrapToken:
            apiServerEndpoint: apiserver.cluster.local:6443
            token: \$1
            unsafeSkipCAVerification: true
    kind: JoinConfiguration
    nodeRegistration:
        criSocket: /run/containerd/containerd.sock
        kubeletExtraArgs:
        container-runtime: remote
        container-runtime-endpoint: unix:///run/containerd/containerd.sock
        taints: null" > \$2/kubeadm.cfg.current
    }
    
    function GenerateStaticNginxProxy() {
        echo "apiVersion: v1
    kind: Pod
    metadata:
      creationTimestamp: null
      name: nginx-proxy
      namespace: kube-system
    spec:
      containers:
      - image: registry.paas/cmss/nginx:1.21.4
        imagePullPolicy: IfNotPresent
        name: nginx-proxy
        resources:
          limits:
            cpu: 300m
            memory: 512M
          requests:
            cpu: 25m
            memory: 32M
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /etc/nginx
          name: etc-nginx
          readOnly: true
      hostNetwork: true
      priorityClassName: system-node-critical
      volumes:
      - hostPath:
          path: /apps/conf/nginx
          type:
        name: etc-nginx
    status: {}" > /etc/kubernetes/manifests/nginx-proxy.yaml
    }
  kubelet_node_helper.sh: |
    #!/usr/bin/env bash
    
    source "env.sh"
    
    # args
    DNS_ADDRESS=\${2:-10.237.0.10}
    LOG_NAME=\${2:-kubelet}
    JOIN_HOST=\$2
    JOIN_TOKEN=\$3
    JOIN_CA_HASH=\$4
    
    function unjoin() {
        # before unjoin, you need delete node by kubectl
        echo "exec(1/5): kubeadm reset...."
        echo "y" | kubeadm reset
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(2/5): restart cotnainerd...."
        systemctl restart containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(3/5): delete cni...."
        if [ -d "/etc/cni/net.d" ]; then
            mv /etc/cni/net.d '/etc/cni/net.d.kosmos.back'\`date +%Y_%m_%d_%H_%M_%S\`
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(4/5): delete ca.crt"
        if [ -f "\$PATH_KUBERNETES_PKI/ca.crt" ]; then
            echo "y" | rm "\$PATH_KUBERNETES_PKI/ca.crt"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(5/5): delete kubelet.conf"
        if [ -f "\$PATH_KUBELET_CONF/\${KUBELET_CONFIG_NAME}" ]; then
            echo "y" | rm "\$PATH_KUBELET_CONF/\${KUBELET_CONFIG_NAME}"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    }
    
    function beforeRevert() {
        if [ -f "/apps/conf/nginx/nginx.conf" ]; then
            # modify  hosts
            config_file="/apps/conf/nginx/nginx.conf"
    
            server_address=\$(grep -Po 'server\s+\K[^:]+(?=:6443)' "\$config_file" | awk 'NR==1')
            hostname=\$(echo \$JOIN_HOST | awk -F ":" '{print \$1}')
            host_record="\$server_address \$hostname"
            if grep -qFx "\$host_record" /etc/hosts; then
                echo "Record \$host_record already exists in /etc/hosts."
            else
                sed -i "1i \$host_record" /etc/hosts
                echo "Record \$host_record inserted into /etc/hosts."
            fi
        fi
    }
    
    function afterRevert() {
        if [ -f "/apps/conf/nginx/nginx.conf" ]; then
            # modify  hosts
            config_file="/apps/conf/nginx/nginx.conf"
    
            server_address=\$(grep -Po 'server\s+\K[^:]+(?=:6443)' "\$config_file" | awk 'NR==1')
            hostname=\$(echo \$JOIN_HOST | awk -F ":" '{print \$1}')
            host_record="\$server_address \$hostname"
            if grep -qFx "\$host_record" /etc/hosts; then
                sudo sed -i "/^\$host_record/d" /etc/hosts
            fi
    
            local_record="127.0.0.1 \$hostname"
            if grep -qFx "\$local_record" /etc/hosts; then
                echo "Record \$local_record already exists in /etc/hosts."
            else
                sed -i "1i \$local_record" /etc/hosts
                echo "Record \$local_record inserted into /etc/hosts."
            fi
    
            GenerateStaticNginxProxy
        fi
    }

    function get_ca_certificate() {
         local output_file="\$PATH_KUBERNETES_PKI/ca.crt"
         local kubeconfig_data=\$(curl -sS --insecure "https://\$JOIN_HOST/api/v1/namespaces/kube-public/configmaps/cluster-info" 2>/dev/null | \
                               \ grep -oP 'certificate-authority-data:\s*\K.*(?=server:[^[:space:]]*?)' | \
                               \  sed -e 's/^certificate-authority-data://' -e 's/[[:space:]]//g' -e 's/\\n$//g')

         # verify the kubeconfig data is not empty
         if [ -z "\$kubeconfig_data" ]; then
           echo "Failed to extract certificate-authority-data."
           return 1
         fi

         # Base64 decoded and written to a file
         echo "\$kubeconfig_data" | base64 --decode > "\$output_file"

         # check that the file was created successfully
         if [ -f "\$output_file" ]; then
             echo "certificate-authority-data saved to \$output_file"
         else
             echo "Failed to save certificate-authority-data to \$output_file"
          return 1
         fi
    }

    function create_kubelet_bootstrap_config() {
       # Checks if the parameters are provided
     if [ -z "\$JOIN_HOST" ] || [ -z "\$JOIN_TOKEN" ]; then
         echo "Please provide server and token as parameters."
         return 1
     fi

     # Define file contents
     cat << EOF > bootstrap-kubelet.conf
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        certificate-authority: \$PATH_KUBERNETES_PKI/ca.crt
        server: https://\$JOIN_HOST
      name: kubernetes
    contexts:
    - context:
        cluster: kubernetes
        user: kubelet-bootstrap
      name: kubelet-bootstrap-context
    current-context: kubelet-bootstrap-context
    preferences: {}
    users:
    - name: kubelet-bootstrap
      user:
        token: \$JOIN_TOKEN
    EOF

     # copy the file to the /etc/kubernetes directory
     cp bootstrap-kubelet.conf \$PATH_KUBERNETES

     echo "the file bootstrap-kubelet.conf has stored in \$PATH_KUBERNETES directory."
    }

    
    function revert() {
        echo "exec(1/5): update kubeadm.cfg..."
        if [ ! -f "\$PATH_KUBEADM_CONFIG/kubeadm.cfg" ]; then
            GenerateKubeadmConfig  \$JOIN_TOKEN \$PATH_FILE_TMP
        else
          sed -e "s|token: .*\$|token: \$JOIN_TOKEN|g" -e "w \$PATH_FILE_TMP/kubeadm.cfg.current" "\$PATH_KUBEADM_CONFIG/kubeadm.cfg"
        fi
    
        # add taints
        echo "exec(2/5): update kubeadm.cfg tanits..."
        sed -i "/kubeletExtraArgs/a \    register-with-taints: node.kosmos.io/unschedulable:NoSchedule"  "\$PATH_FILE_TMP/kubeadm.cfg.current"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(3/5): update kubelet-config..."
        sed -e "s|__DNS_ADDRESS__|\$HOST_CORE_DNS|g" -e "w \${PATH_KUBELET_CONF}/\${KUBELET_CONFIG_NAME}" "\$PATH_FILE_TMP"/"\$KUBELET_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
    #    beforeRevert
    #    if [ \$? -ne 0 ]; then
    #        exit 1
    #    fi
    
    
        echo "exec(4/5): execute join cmd...."
        kubeadm join --config "\$PATH_FILE_TMP/kubeadm.cfg.current"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(5/5): restart cotnainerd...."
        systemctl restart containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
    #    afterRevert
    #    if [ \$? -ne 0 ]; then
    #        exit 1
    #    fi
    
    
    }
    
    # before join, you need upload ca.crt and kubeconfig to tmp dir!!!
    function join() {
        echo "exec(1/8): stop containerd...."
        systemctl stop containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(2/8): copy ca.crt...."
        cp "\$PATH_FILE_TMP/ca.crt" "\$PATH_KUBERNETES_PKI/ca.crt"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(3/8): copy kubeconfig...."
        cp "\$PATH_FILE_TMP/\$KUBELET_KUBE_CONFIG_NAME" "\$PATH_KUBERNETES/\$KUBELET_KUBE_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(4/8): set core dns address...."
        sed -e "s|__DNS_ADDRESS__|\$DNS_ADDRESS|g" -e "w \${PATH_KUBELET_CONF}/\${KUBELET_CONFIG_NAME}" "\$PATH_FILE_TMP"/"\$KUBELET_CONFIG_NAME"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
        echo "exec(5/8): copy kubeadm-flags.env...."
        cp "\$PATH_FILE_TMP/kubeadm-flags.env" "\$PATH_KUBELET_LIB/kubeadm-flags.env"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(6/8): delete cni...."
        if [ -d "/etc/cni/net.d" ]; then
            mv /etc/cni/net.d '/etc/cni/net.d.back'\`date +%Y_%m_%d_%H_%M_%S\`
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "exec(7/8): start containerd"
        systemctl start containerd
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "exec(8/8): start kubelet...."
        systemctl start kubelet
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    }
    
    function health() {
        result=\`systemctl is-active containerd\`
        if [[ \$result != "active" ]]; then
            echo "health(1/2): containerd is inactive"
            exit 1
        else
            echo "health(1/2): containerd is active"
        fi
    
        result=\`systemctl is-active kubelet\`
        if [[ \$result != "active" ]]; then
            echo "health(2/2): kubelet is inactive"
            exit 1
        else
            echo "health(2/2): containerd is active"
        fi
    }
    
    function log() {
        systemctl status \$LOG_NAME
    }
    
    # check the environments
    function check() {
        # TODO: create env file
        echo "check(1/2): try to create \$PATH_FILE_TMP"
        if [ ! -d "\$PATH_FILE_TMP" ]; then
            mkdir -p "\$PATH_FILE_TMP"
            if [ \$? -ne 0 ]; then
                exit 1
            fi
        fi
    
        echo "check(2/2): copy  kubeadm-flags.env  to create \$PATH_FILE_TMP , remove args[cloud-provider] and taints"
        sed -e "s| --cloud-provider=external | |g" -e "w \${PATH_FILE_TMP}/kubeadm-flags.env" "\$PATH_KUBELET_LIB/kubeadm-flags.env"
        sed -i "s| --register-with-taints=node.kosmos.io/unschedulable:NoSchedule||g" "\${PATH_FILE_TMP}/kubeadm-flags.env"
        if [ \$? -ne 0 ]; then
            exit 1
        fi
    
        echo "environments is ok"
    }
    
    function version() {
        echo "\$SCRIPT_VERSION"
    }
    
    # See how we were called.
    case "\$1" in
        unjoin)
        unjoin
        ;;
        join)
        join
        ;;
        health)
        health
        ;;
        check)
        check
        ;;
        log)
        log
        ;;
        revert)
        revert
        ;;
        version)
        version
        ;;
        *)
        echo $"usage: \$0 unjoin|join|health|log|check|version|revert"
        exit 1
    esac
  config.yaml: |
    apiVersion: kubelet.config.k8s.io/v1beta1
    authentication:
      anonymous:
        enabled: false
      webhook:
        cacheTTL: 0s
        enabled: true
      x509:
        clientCAFile: /etc/kubernetes/pki/ca.crt
    authorization:
      mode: Webhook
      webhook:
        cacheAuthorizedTTL: 0s
        cacheUnauthorizedTTL: 0s
    cgroupDriver: systemd
    clusterDNS:
    - __DNS_ADDRESS__
    clusterDomain: cluster.local
    cpuManagerReconcilePeriod: 0s
    evictionHard:
      imagefs.available: 15%
      memory.available: 100Mi
      nodefs.available: 10%
      nodefs.inodesFree: 5%
    evictionPressureTransitionPeriod: 5m0s
    fileCheckFrequency: 0s
    healthzBindAddress: 127.0.0.1
    healthzPort: 10248
    httpCheckFrequency: 0s
    imageMinimumGCAge: 0s
    kind: KubeletConfiguration
    kubeAPIBurst: 100
    kubeAPIQPS: 100
    kubeReserved:
      cpu: 140m
      memory: 1.80G
    logging:
      flushFrequency: 0
      options:
        json:
          infoBufferSize: "0"
      verbosity: 0
    memorySwap: {}
    nodeStatusReportFrequency: 0s
    nodeStatusUpdateFrequency: 0s
    rotateCertificates: true
    runtimeRequestTimeout: 0s
    shutdownGracePeriod: 0s
    shutdownGracePeriodCriticalPods: 0s
    staticPodPath: /etc/kubernetes/manifests
    streamingConnectionIdleTimeout: 0s
    syncFrequency: 0s
    volumeStatsAggPeriod: 0s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: virtual-cluster-operator
  namespace: kosmos-system
  labels:
    app: virtual-cluster-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: virtual-cluster-operator
  template:
    metadata:
      labels:
        app: virtual-cluster-operator
    spec:
      # Enter the name of the node where the virtual cluster operator is deployed
      nodeName: kubenest-control-plane
      serviceAccountName: virtual-cluster-operator
      tolerations:
        - key: "node-role.kubernetes.io/control-plane"
          operator: "Exists"
          effect: "NoSchedule"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - virtual-cluster-operator
                topologyKey: kubernetes.io/hostname
      containers:
        - name: virtual-cluster-operator
          image: ${IMAGE_REPOSITIRY}/virtual-cluster-operator:latest
          imagePullPolicy: Always
          env:
            - name: WAIT_NODE_READ_TIME
              value: "120"
            - name: IMAGE_REPOSITIRY
              value: ${IMAGE_REPOSITIRY}
            - name: IMAGE_VERSION
              value: v1.25.7-eki.3.0.0
            - name: EXECTOR_HOST_MASTER_NODE_IP
              value: $control_plane_ip
            - name: EXECTOR_SHELL_NAME
              value: kubelet_node_helper_1.sh
            - name: WEB_USER
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: username
            - name: WEB_PASS
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: password
          volumeMounts:
          - name: credentials
            mountPath: /etc/virtual-cluster-operator
            readOnly: true
          - name: shellscript
            mountPath: /etc/vc-node-dir/env.sh
            subPath: env.sh
          - name: shellscript
            mountPath: /etc/vc-node-dir/kubelet_node_helper_1.sh
            subPath: kubelet_node_helper.sh
          - name: shellscript
            mountPath: /etc/vc-node-dir/config.yaml
            subPath: config.yaml
          - mountPath: /kosmos/manifest
            name: components-manifest
          command:
          - virtual-cluster-operator
          - --kubeconfig=/etc/virtual-cluster-operator/kubeconfig
          - --kube-nest-anp-mode=uds
          - --v=4
      volumes:
        - name: credentials
          secret:
            secretName: virtual-cluster-operator
        - name: shellscript
          configMap:
            name: virtual-cluster-operator
        - hostPath:
            path: /apps/vc-operator/manifest
            type: DirectoryOrCreate
          name: components-manifest
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: node-agent
  namespace: kosmos-system
spec:
  selector:
    matchLabels:
      app: node-agent-service
  template:
    metadata:
      labels:
        app: node-agent-service
    spec:
      hostPID: true # access host pid
      hostIPC: true # access host ipc
      hostNetwork: true # access host network
      tolerations:
        - operator: Exists # run on all nodes
      initContainers:
        - name: init-agent
          image: ${IMAGE_REPOSITIRY}/node-agent:latest
          securityContext:
            privileged: true
          env:
            - name: WEB_USER
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: username
            - name: WEB_PASS
              valueFrom:
                secretKeyRef:
                  name: node-agent-secret
                  key: password
          command: ["/bin/bash"]
          args:
            - "/app/init.sh"
          volumeMounts:
            - mountPath: /host-path
              name: node-agent
              readOnly: false
            - mountPath: /host-systemd
              name: systemd-path
              readOnly: false
      containers:
        - name: install-agent
          image: ${IMAGE_REPOSITIRY}/node-agent:latest
          securityContext:
            privileged: true # container privileged
          command:
            - nsenter
            - --target
            - "1"
            - --mount
            - --uts
            - --ipc
            - --net
            - --pid
            - --
            - bash
            - -l
            - -c
            - "/srv/node-agent/start.sh && sleep infinity"
      volumes:
        - name: node-agent
          hostPath:
            path: /srv/node-agent
            type: DirectoryOrCreate
        - name: systemd-path
          hostPath:
            path: /etc/systemd/system
            type: DirectoryOrCreate
---
apiVersion: v1
kind: Secret
metadata:
  name: node-agent-secret
  namespace: kosmos-system
type: kubernetes.io/basic-auth
stringData:
  username: "kosmos-node-agent"
  password: "bdp_dspt_202X_pA@Min1a"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kosmos-hostports
  namespace: kosmos-system
data:
  config.yaml: |
    # ports allocate for virtual cluster api server,from 33001, increment by 1 for each virtual cluster.Be careful not to use ports that are already in use
    portsPool:
      - 33001
      - 33002
      - 33003
      - 33004
      - 33005
      - 33006
      - 33007
      - 33008
      - 33009
      - 33010
      - 33011
      - 33012
      - 33013
      - 33014
      - 33015
      - 33016
      - 33017
      - 33018
      - 33019
      - 33020
      - 33021
      - 33022
      - 33023
      - 33024
      - 33025
      - 33026
      - 33027
      - 33028
      - 33029
      - 33030
      - 33031
      - 33032
      - 33033
      - 33034
      - 33035
      - 33036
      - 33037
      - 33038
      - 33039
      - 33040
      - 33041
      - 33042
      - 33043
      - 33044
      - 33045
      - 33046
      - 33037
      - 33048
      - 33049
      - 33050
---
apiVersion: v1
data:
  components: |
    [
      {"name": "kube-proxy", "path": "/kosmos/manifest/kube-proxy/*.yaml"},
      {"name": "calico", "path": "/kosmos/manifest/calico/*.yaml"},
      {"name": "keepalived", "path": "/kosmos/manifest/keepalived/*.yaml"},
    ]
  host-core-dns-components: |
    [
      {"name": "core-dns-host", "path": "/kosmos/manifest/core-dns/host/*.yaml"},
    ]
  virtual-core-dns-components: |
    [
      {"name": "core-dns-virtual", "path": "/kosmos/manifest/core-dns/virtualcluster/*.yaml"},
    ]
kind: ConfigMap
metadata:
  name: components-manifest-cm
  namespace: kosmos-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kosmos-vip-pool
  namespace: kosmos-system
data:
  vip-config.yaml: |
    # can be use for vc, the ip formate is 192.168.0.1 and 192.168.0.2-192.168.0.10
    vipPool:
      - 192.168.6.110-192.168.6.120
EOF

kubectl apply -f vc-operator.yaml
kubectl apply -f ${GLOBALNODES_YAML_PATH}
kubectl apply -f ${CRDS_YAML_PATH}

# 检查所有Pod是否为Running状态
all_pods_running() {
  local not_ready_pods=$(kubectl get pods -n kosmos-system --no-headers | grep -v 'Running\|Completed')

  if [ -z "$not_ready_pods" ]; then
    return 0 # 所有Pod都是Running或Completed状态
  else
    echo "以下Pod未处于Running状态:"
    echo "$not_ready_pods"
    return 1 # 仍有Pod未处于Running状态
  fi
}

# 循环等待，直到所有Pod都为Running状态
while ! all_pods_running; do
  echo "等待所有Pod进入Running状态..."
  sleep 5 # 等待5秒再检查一次
done

echo "所有Pod已准备好。"

echo -n >nodes.txt
# 列出所有 worker 节点并让用户选择一个节点
PS3="请输入数字选择节点: "
select worker_node in $(kubectl get nodes --selector='!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[*].metadata.name}'); do
  if [ -n "$worker_node" ]; then
    echo "你选择了节点: $worker_node"
    echo $worker_node >nodes.txt
    wait_for_step "选择 worker 节点并保存到 nodes.txt"
    break
  else
    echo "无效的选择，请重新选择。"
  fi
done

read -r worker < nodes.txt
echo ${worker}

bash ${GLOBALNODES_HELPER_PATH} free
cat <<EOF >vc-test.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test
---
apiVersion: kosmos.io/v1alpha1
kind: VirtualCluster
metadata:
  name: vc1
  namespace: test
spec:
  externalIP: 192.168.0.1
  kubeInKubeConfig:
    tenantEntrypoint:
      externalVips:
        - 192.168.0.2  # 添加 ExternalVips 的值
  promotePolicies:
    - labelSelector:
        matchLabels:
          kubernetes.io/hostname: ${worker}
      nodeCount: 1
EOF

kubectl apply -f vc-test.yaml
kubectl get vc -A

rm ${REPO_ROOT}/hack/kind-k8s-in-k8s-config.yaml
rm ${REPO_ROOT}/hack/vc-operator.yaml
rm ${REPO_ROOT}/hack/vc-test.yaml
rm ${REPO_ROOT}/hack/openebs-hostpath.yaml
