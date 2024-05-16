# Summary

At present, when creating a virtual cluster in Kubernetes, it is necessary to create plugins within the cluster based on different scenarios. Besides the built-in components of k8s, we cannot hardcode other enhancements like node-local-dns, apiserver-network-proxy, etc. It is necessary to provide a plugin-based approach for installing these.
# Motivation

Offer a pluggable installation method for cluster components

## Goals

Use CRDs to describe plugins, with each plugin represented by an individual CR. When creating a cluster, you can specify in the virtualCluster which plugins need to be installed or not. The installation of plugins specified in the virtualCluster begins when the cluster's status reaches AllNodeReady. After all plugins are installed (how to determine if a plugin installation is successful will be set aside for now), the status of the virtualCluster changes to completed.
## Non-Goals

# Proposal

The plugin-based installation method for the Kube in kube solution


## Function overview

1. Specify the plugins to be installed or not installed through virtualCluster

2. Define the plugin's CRD, supporting various methods of plugin installation

3. After determining the plugin installation is successful, set the cluster's status to completed

## User Stories (Optional)

## story 1

# Design Details

## Overall structure

![image.png](img/image.png)

virtual-cluster-operator: Monitors the VirtualCluster resource and creates the control plane and Plugin-executor-job

webhook: Dynamically injects the yaml of Plugin-executor-job based on the configuration of virtualCluster and VirtualClusterPlugin, injecting pv or hostpath into it

Plugin-executor-job: ansible-operator, built-in helm, kustomize, kubectl, and other tools, responsible for installing the plugins configured by VirtualClusterPlugin, and determines whether the installation is successful based on the commands of successCommand. If it is not filled in, then it is not judged.

Plugin: plugin is a crd, configuring the storage location and parameters of yaml or helm files

storage: storage is the storage location of the plugin files, including hostpath, pv, object storage, url, and other storage methods

## design of CRD

一、VirtualCluster adds fields related to plugins

```YAML
apiVersion: kosmos.io/v1alpha1
kind: VirtualCluster
metadata:
  name: tenant1-cluster
spec:
  kubeconfig: XXXXXX
  promoteResources:
    nodes:
    - node1
    - node2
    resources:
      cpu: 10
      memory: 100Gi
  pluginSet:
    enabled:
      - name: pluginA
      - name: pluginB
    disabled:
      - name: pluginC

```

Specify the plugins to install and not to install through pluginSet

二、Add CRD for VirtualClusterPlugin

```YAML
apiVersion: kosmos.io/v1alpha1
kind: VirtualClusterPlugin
metadata:
  name: apiserver-network-proxy
spec:
  successStateCommand: "kubectl get pods -n default -l app=test --field-selector=status.phase!=Running | grep -q 'No resources found' && echo "所有的Pod都处于Running状态。" || echo "有Pod不处于Running状态。""
  pluginSources:                    
    chart:                          
      name: xxx             
      repo:  xxx             
      storage: 
        pvPath: /root
        hostPath: /root
        uri: https://XXXX.yaml
      values:  xxx           
      valuesFile: 
        pvPath: /root
        hostPath: /root
        uri: https://XXXX.yaml
    yaml: 
      path:            
        pvPath: /root
        hostPath:
          path: /root
          nodeName: node1
        uri: https://XXXX.yaml
```

## Overall process

For example, using the pv method to store deployment files

1. Create a service that binds to a pv

```YAML
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ubuntu-statefulset
spec:
  serviceName: "ubuntu"
  replicas: 1
  selector:
    matchLabels:
      app: ubuntu
  template:
    metadata:
      labels:
        app: ubuntu
    spec:
      containers:
      - name: ubuntu
        image: ubuntu
        command: ["sleep", "infinity"]
        volumeMounts:
        - mountPath: "/pv"
          name: plugin-store
  volumeClaimTemplates:
  - metadata:
      name: plugin-store
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: "openebs-hostpath"
      resources:
        requests:
          storage: 1Gi

```

1. Check pv

```Shell
pvc-bdac7fb6-a311-46a1-85ea-85099372a73e   1Gi        RWO            Delete           Bound    default/plugin-store-ubuntu-statefulset-0   openebs-hostpath            8s
```

1. Copy deployment files to pv

```Shell
kubectl cp test.yaml ubuntu-statefulset-0:/pv
```

1. create plugin

```YAML
apiVersion: kosmos.io/v1alpha1
kind: VirtualClusterPlugin
metadata:
  name: apiserver-network-proxy
spec:
  pluginSources:                    
    yaml: 
      path:            
        pvPath: plugin-store-ubuntu-statefulset-0:/pv
```

This side does not have a successCommand set therefore when the virtual-cluster-operator is deploying it applies directly without determining success based on the successCommand

If there is a need to dynamically pass parameters for the deployment of plugins, one must choose either helm or customize methods Yaml methods only support direct application and do not support dynamic parameter passing

1. Overall process analysis

When users arrive at a new environment they pre-create the plugin

Subsequently, when creating the VirtualCluster's CR, the operator will install components based on the plugin configured in the vc During installation, the installation of each plugin will create a job named plugin-executor-job to install the plugin

Example of a job

```Shell
apiVersion: batch/v1
kind: Job
metadata:
  name: apply-yaml-job
spec:
  templat
    spec:
      containers:
      - name: kubectl-container
        image: bitnami/kubectl  
        command: ["kubectl", "apply", "-f", "/mnt/example.yaml"]  
        volumeMounts:
        - name: mypvc
          mountPath: /pv
      restartPolicy: Never
      volumes:
      - name: mypvc
        persistentVolumeClaim:
          claimName: plugin-store-ubuntu-statefulset-0
  backoffLimit: 4
```

If a plugin has set successCommand, then the start command needs to first complete the deployment task followed by adding a wait for the successCommand command.

After all the plugin's jobs have been executed, update the VirtualCluster's status to completed.

# Test plan

## Unit Test

## E2E Test



