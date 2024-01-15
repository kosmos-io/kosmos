# Clustertree Node Resources

## 概要

- 添加 `node_resources_controller` 控制器到`KOSMOS ClusterTree`模块。用于监控和调节集群中节点资源的使用情况。
- 添加 `node_lease_controller` 控制器到`KOSMOS ClusterTree`模块。用于管理节点租约，确保节点的租约在`Leaf`集群和`Root`集群之间同步更新。

## 动机
- 将Leaf集群中的节点资源与`Root`集群中的节点资源同步，使得`Root`集群中的资源调度更加精准。

### 目标
- 确保`Leaf`集群节点资源与`Root`集群维护的节点信息实时同步和一致。
- 确保`Leaf`集群和`Root`集群之间的节点租约同步，保证集群的高可用性和稳定性。

### 非目标

## 提议

## 设计细节

### 架构图
![clustertree_node architecture](img/clustertree-node.png)

### 节点资源
#### API定义
添加配置到 `NodeResourcesController`
```go
type NodeResourcesController struct {
	Leaf              client.Client
	Root              client.Client
	GlobalLeafManager leafUtils.LeafResourceManager
	RootClientset     kubernetes.Interface

	Nodes             []*corev1.Node
	LeafNodeSelectors map[string]kosmosv1alpha1.NodeSelector
	LeafModelHandler  leafUtils.LeafModelHandler
	Cluster           *kosmosv1alpha1.Cluster
	EventRecorder     record.EventRecorder
}
```

#### 初始化
在`SetupWithManager`函数中，通过`builder.WithPredicates`方法指定了一个`predicate.Funcs`类型的对象`predicatesFunc`，用于过滤需要处理的事件。在这个例子中，我们只关注Node对象的创建、更新和删除事件。
```go
func (c *NodeResourcesController) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr). 
		Named(NodeResourcesControllerName). // NewControllerManagedBy方法创建控制器，指定管理器mgr和控制器名称NodeResourcesControllerName：node-resources-controller。
		WithOptions(controller.Options{}). // WithOptions方法设置控制器选项，这里为空。
		For(&corev1.Node{}, builder.WithPredicates(predicatesFunc)). // For方法指定要处理的资源类型，这里是corev1.Node对象，并将过滤器predicatesFunc传入。
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(c.podMapFunc())). // 通过Watches方法监听与Pod对象相关的事件，并指定一个MapFunc函数podMapFunc来处理这些事件。这里的处理逻辑是，如果Pod的Spec中有指定NodeName，就将对应的节点名作为参数构建一个reconcile.Request对象，并加入到返回值列表中。
		Complete(c) // 完成控制器的配置。
}
```

#### 调和逻辑
`Reconcile`函数中，实现了具体的调和逻辑：
- 遍历所有的`Root`节点，获取每个节点的信息；
- 通过`LeafModelHandler`接口的方法，获取`Leaf`集群中与当前`Root`节点对应的`Leaf`节点和`Pod`列表；
- 根据获取到的信息，计算集群资源的分配情况；
- 使用`CreateMergePatch`函数生成要对`Root`节点进行更新的补丁；
- 使用`CoreV1`接口的`Patch`方法对`Root`节点进行更新；
- 返回空的`Result`表示处理成功。

### 节点租约
#### API定义
添加配置到 `NodeLeaseController`
```go
type NodeLeaseController struct {
	leafClient       kubernetes.Interface
	rootClient       kubernetes.Interface
	root             client.Client
	LeafModelHandler leafUtils.LeafModelHandler

	leaseInterval  time.Duration
	statusInterval time.Duration

	nodes             []*corev1.Node
	LeafNodeSelectors map[string]kosmosv1alpha1.NodeSelector
	nodeLock          sync.Mutex
}
```

#### 初始化
在`Start`函数中，启动两个goroutine分别用于同步租约`syncLease`和节点状态`syncNodeStatus`。
```go
func (c *NodeLeaseController) Start(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.syncLease, c.leaseInterval) // 在syncLease函数中，首先检查与Leaf集群的连接状态，然后创建租约（如果不存在），并尝试更新租约。
	go wait.UntilWithContext(ctx, c.syncNodeStatus, c.statusInterval)
	<-ctx.Done()
	return nil
}
```

#### 调和逻辑
`Reconcile`函数中，实现了具体的调和逻辑：
- 在`Start`函数中，使用`go`关键字启动两个`goroutine`，分别调用`syncLease`和`syncNodeStatus`函数。
- 在`syncLease`函数中，首先使用`leafClient`检查与`Leaf`集群的连接状态，如果连接失败，则直接返回。
- 然后对每个节点执行以下操作：
  - 使用`rootClient`和节点的名称构造一个`namespaceName`对象，用于获取租约。
  - 使用`rootClient`根据`namespaceName`获取租约对象。
  - 如果获取到的租约对象不存在，则根据节点信息创建一个新的租约对象，并使用`rootClient`将其创建到集群中。
  - 如果获取到的租约对象存在，则更新租约对象的`RenewTime`字段，并使用`rootClient`将其更新到集群中。
- 在`syncNodeStatus`函数中，首先获取所有节点的副本（为了避免并发修改原始节点列表），然后调用`updateNodeStatus`函数更新节点状态。
- 在`updateNodeStatus`函数中，调用`LeafModelHandler`接口的`UpdateRootNodeStatus`函数更新`Root`集群中的节点状态。

### Test Plan