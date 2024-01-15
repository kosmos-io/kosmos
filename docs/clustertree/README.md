# ClusterTree(集群容器编排)

## 技术架构图
<div><img src="../images/clustertree-arch.png" style="width:900px;"  alt="ClusterTree"/></div>

- 所有的服务部署都需要在kosmos控制面上执行，通过kosmos的控制面将工作负载（deploy、service、PV、PVCS）
- 所有的controller都是通过Cluster-Manager这个服务来进行管理
- 整个ClusterTree暂时只用到Cluster这个CRD，这个CRD中主要存储整个kosmos集群中各个集群的kubeconfig配置信息
- controller通过读取Cluster 这个CRD来读取kubeconfig然后进行调和

## 详细介绍
### 如何进行本地debug
- 推荐使用goland进行开发和测试
- 使用kind创建3个集群，具体参考[README中的Quick Start](../../README.md#quick-start)部分内容
- 配置Program Arguments 为 
`--kube-qps=500 --kube-burst=1000 --kubeconfig=$ProjectFileDir$/kube-config/kosmos-control-cluster-config.yaml --leader-elect=false --multi-cluster-service=false --v=4`
将`--kubeconfig`改成kosmos主集群配置
- 由于kind创建的3个集群，是通过内部的coredns进行通信，所以在Cluster-Manager进行debug或者运行的时候，需要改下config的获取，并修改IP和端口信息

### Cluster-Manger
- 整体的模块拆分如下
- controller目录包含pod、pv、pvc的调和以及跨集群的服务导入、服务导出（实现服务的注册和服务发现）
- extensions目录包含kosmos自身daemonset处理的controller，实现kosmos daemonset下发到不同的集群
- node-server目录主要功能为管理整个集群的node，通过http的api提供一些服务，比如执行命令、获取日志

```text
└── cluster-manager
    ├── cluster_controller.go
    ├── controllers
    │   ├── common_controller.go
    │   ├── mcs
    │   │   ├── auto_mcs_controller.go
    │   │   ├── serviceexport_controller.go
    │   │   └── serviceimport_controller.go
    │   ├── node_lease_controller.go
    │   ├── node_resources_controller.go
    │   ├── pod
    │   │   ├── leaf_pod_controller.go
    │   │   ├── root_pod_controller.go
    │   │   └── storage_handler.go
    │   ├── pv
    │   │   ├── leaf_pv_controller.go
    │   │   ├── oneway_pv_controller.go
    │   │   └── root_pv_controller.go
    │   └── pvc
    │       ├── leaf_pvc_controller.go
    │       ├── oneway_pvc_controller.go
    │       └── root_pvc_controller.go
    ├── extensions
    │   └── daemonset
    │       ├── constants.go
    │       ├── daemonset_controller.go
    │       ├── daemonset_mirror_controller.go
    │       ├── distribute_controller.go
    │       ├── host_daemon_controller.go
    │       ├── pod_reflect_controller.go
    │       ├── update.go
    │       └── utils.go
    ├── node-server
    │   ├── api
    │   │   ├── errdefs.go
    │   │   ├── exec.go
    │   │   ├── helper.go
    │   │   ├── logs.go
    │   │   └── remotecommand
    │   │       ├── attach.go
    │   │       ├── exec.go
    │   │       ├── httpstream.go
    │   │       └── websocket.go
    │   └── server.go
    └── utils
        ├── leaf_model_handler.go
        ├── leaf_resource_manager.go
        └── rootcluster.go

```