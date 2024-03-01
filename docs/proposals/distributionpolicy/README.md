# Kosmos Scheduler plugins

## Summary
Kosmos provides a multi-cluster scheduler (kosmos-scheduler) and some scheduling plugins.

## Motivation & User Stories
1、`distributionpolicy`: During the product migration process, we hope that the pods created by the user operator can be scheduled to the member cluster without modifying the user's code. Therefore, we provide the distributionpolicy plugin to schedule to the desired nodes (host node, leaf node, mix node) according to matching rules (namespace, labelselector, name and other fields)

## Design Details
### DistributionPolicy & ClusterDistributionPolicy
#### CRD API
[Code](https://github.com/kosmos-io/kosmos/pull/321) responsible for working with DistributionPolicy and ClusterDistributionPolicy CRD API will be imported in the kosmos-scheduler plugins repo. DistributionPolicy is namespace scope and ClusterDistributionPolicy is cluster scope.

#### Filter extension points implementation details
Since target resources distribution policies are stored in the CRD (DistributionPolicy & ClusterDistributionPolicy), kosmos-scheduler should be subscribed for updates of appropriate CRD type. kosmos-scheduler will use informers which will be generated with the name dpInformer(cdpInformer). CRD will contian in ResourceSelectors and PolicyTerms. ResourceSelectors used to select resources and is required. PolicyTerms represents the rule for select nodes to distribute resources.

**Description of the ResourceSelectors rules**
1. ResourceSelectors is required
2. Prioritize matching of high-precision rule
In case of two ResourceSelector, the one with a more precise
matching rules in ResourceSelectors wins:
- Namespace-scope resources have higher priority  than cluster-scope resources
- For namespace-scope resources, matching by name (resourceSelector.name) has higher priority than by namePrefix (resourceSelector.namePrefix)
- For namespace-scope resources, matching by namePrefix (resourceSelector.namePrefix) has higher priority than by selector(resourceSelector.labelSelector) 
- For cluster-scope resources, matching by name (resourceSelector.name) has higher priority than by namePrefix (resourceSelector.namePrefix)
- For cluster-scope resources, matching by NamePrefix (resourceSelector.namePrefix) has higher priority than by selector(resourceSelector.labelSelector)
The more the precise, the higher the priority. Defaults to zero which means schedule to the mix node.

**PolicyTerms**
1. PolicyTerms is required
2. The current node scheduling policy is divided into four nodeTypes （host, leaf, mix, adv）.
3. Advanced options will be supported in the future. Sure as NodeSelector, Affinity and so on.

## Use cases
### DistributionPolicy CRD
```yaml
apiVersion: kosmos.io/v1alpha1
kind: DistributionPolicy
metadata:
  name: kosmos-node-distribution-policy
  namespace: test-name-scope
spec:
  resourceSelectors:
    - name: busybox
      policyName: leaf
    - namePrefix: busybox-prefix
      policyName: host
    - labelSelector:
        matchLabels:
          example-distribution-policy: busybox
      policyName: mix
  policyTerms:
  - name: host
    nodeType: host
  - name: leaf
    nodeType: leaf
  - name: mix
    nodeType: mix
  - name: adv
    nodeType: adv
    advancedTerm:
      nodeSelector:
        advNode: "true"
```
### ClusterDistributionPolicy CRD
```yaml
apiVersion: kosmos.io/v1alpha1
kind: ClusterDistributionPolicy
metadata:
  name: kosmos-node-cluster-distribution-policy
spec:
  resourceSelectors:
    - name: cluster-busybox
      policyName: leaf
    - namePrefix: cluster-busybox-prefix
      policyName: host
    - labelSelector:
        matchLabels:
          example-distribution-policy: cluster-busybox
      policyName: mix
  policyTerms:
  - name: host
    nodeType: host
  - name: leaf
    nodeType: leaf
  - name: mix
    nodeType: mix
  - name: adv
    nodeType: adv
    advancedTerm:
      nodeName: kosmos-member2-cluster-1
```
## Test plans
### Preparatory Work
First, Kosmos needs to be [deployed successfully](https://mp.weixin.qq.com/s/6zZXPP9FKbgWV1JUYv-iVw) (at least the clustertree module is deployed) and join the member cluster correctly.
### Deploy the Kosmos-scheduler
1. Configure scheduler and scheduling policy
```shell
kubectl apply -f kosmos/deploy/scheduler/.
```
2. Verify the kosmos-scheduler service
```shell
kubectl -n kosmos-system get pod
NAME                                         READY   STATUS    RESTARTS   AGE
kosmos-scheduler-8f96d87d7-ssxrx             1/1     Running   0          24s
```
### Try a Sample
1. Use case yaml（busybox.yaml）
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox
  namespace: test-name-scope
spec:
  replicas: 3
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
        example-distribution-policy: busybox
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: busybox
              topologyKey: kubernetes.io/hostname
      schedulerName: kosmos-scheduler
      containers:
        - name: busybox
          image: busybox:latest
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
            - sleep 3000
```
3. Other Operation instructions
```shell
# List all nodes in the host cluster
kubectl get node
NAME                       STATUS     ROLES                       AGE   VERSION
kosmoscluster1-1           Ready      control-plane,master,node   21d   v1.21.5-eki.0
kosmoscluster1-2           Ready      node                        21d   v1.21.5-eki.0
kosmos-member2-cluster-1   Ready      agent                       24m   v1.21.5-eki.0
kosmos-member2-cluster-2   Ready      agent                       24m   v1.21.5-eki.0
 
# Show the taint information on the virtual node
kubectl describe node kosmos-member2-cluster-1  |grep Taints
Taints:             node.kubernetes.io/unreachable:NoExecute
 
kubectl describe node kosmos-member2-cluster-2  |grep Taints
Taints:             node.kubernetes.io/unreachable:NoExecute
 
# Scheduling by the kosmos-scheduler (hybrid scheduling)
kubectl apply -f  busybox.yaml
    
# Show instances (hybrid) scheduling result in host cluster
kubectl get pod -owide -n test-name-scope
NAME                       READY   STATUS    RESTARTS   AGE   IP              NODE                       NOMINATED NODE   READINESS GATES
busybox-69855845c9-2pl7f   1/1     Running   0          14s   10.xx.xx.12     kosmoscluster1-1           <none>           <none>
busybox-69855845c9-54cm9   1/1     Running   0          14s   10.xx.xx.92     kosmoscluster1-2           <none>           <none>
busybox-69855845c9-9gjs9   1/1     Running   0          14s   10.xx.xx.80     kosmos-member2-cluster-1   <none>           <none>

# Show instances (hybrid) scheduling result in member cluster
kubectl get pod -owide -n test-name-scope
NAME                       READY   STATUS    RESTARTS   AGE   IP              NODE                       NOMINATED NODE   READINESS GATES
busybox-69855845c9-9gjs9   1/1     Running   0          14s   10.xx.xx.80     kosmos-member2-cluster-1   <none>           <none>
```
