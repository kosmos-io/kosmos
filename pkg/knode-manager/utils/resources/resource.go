package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

type CustomResources map[corev1.ResourceName]resource.Quantity

// DeepCopy copy the custom resource
func (cr CustomResources) DeepCopy() CustomResources {
	crCopy := CustomResources{}
	for name, quota := range cr {
		crCopy[name] = quota
	}
	return crCopy
}

// Equal return if resources is equal
func (cr CustomResources) Equal(other CustomResources) bool {
	if len(cr) != len(other) {
		return false
	}
	for k, v := range cr {
		v1, ok := other[k]
		if !ok {
			return false
		}
		if !v1.Equal(v) {
			return false
		}
	}
	return true
}

type Resource struct {
	CPU              resource.Quantity
	Memory           resource.Quantity
	Pods             resource.Quantity
	EphemeralStorage resource.Quantity
	Custom           CustomResources
}

func NewResource() *Resource {
	return &Resource{
		Custom: CustomResources{},
	}
}

func (r *Resource) Equal(other *Resource) bool {
	return r.CPU.Equal(other.CPU) && r.Memory.Equal(other.Memory) && r.Pods.Equal(other.Pods) && r.
		EphemeralStorage.Equal(other.EphemeralStorage) && r.Custom.Equal(other.Custom)
}

func (r *Resource) Add(nc *Resource) {
	r.CPU.Add(nc.CPU)
	r.Memory.Add(nc.Memory)
	r.Pods.Add(nc.Pods)
	r.EphemeralStorage.Add(nc.EphemeralStorage)
	if len(nc.Custom) == 0 {
		return
	}
	for name, quota := range nc.Custom {
		if r.Custom == nil {
			r.Custom = CustomResources{}
		}
		old := r.Custom[name]
		old.Add(quota)
		r.Custom[name] = old
	}
}

func (r *Resource) Sub(nc *Resource) {
	r.CPU.Sub(nc.CPU)
	r.Memory.Sub(nc.Memory)
	r.Pods.Sub(nc.Pods)
	r.EphemeralStorage.Sub(nc.EphemeralStorage)
	if len(nc.Custom) == 0 {
		return
	}
	for name, quota := range nc.Custom {
		if r.Custom == nil {
			r.Custom = CustomResources{}
		}
		old := r.Custom[name]
		old.Sub(quota)
		r.Custom[name] = old
	}
}

func (r *Resource) SetResourcesToNode(node *corev1.Node) {
	var CPU, mem, Pods, empStorage resource.Quantity
	if !r.CPU.IsZero() {
		CPU = r.CPU
	}
	if !r.Memory.IsZero() {
		mem = r.Memory
	}
	if !r.Pods.IsZero() {
		Pods = r.Pods
	}
	if !r.EphemeralStorage.IsZero() {
		empStorage = r.EphemeralStorage
	}
	node.Status.Capacity = corev1.ResourceList{
		corev1.ResourceCPU:              CPU,
		corev1.ResourceMemory:           mem,
		corev1.ResourcePods:             Pods,
		corev1.ResourceEphemeralStorage: empStorage,
	}
	for name, quota := range r.Custom {
		node.Status.Capacity[name] = quota
	}

	node.Status.Allocatable = node.Status.Capacity.DeepCopy()
	klog.Infof("%v", node.Status.Capacity)
}

func ConvertToResource(resources corev1.ResourceList) *Resource {
	var cpu, mem, pods, empStorage resource.Quantity
	customResource := CustomResources{}
	for resourceName, quota := range resources {
		switch resourceName {
		case corev1.ResourceCPU:
			cpu = quota
		case corev1.ResourceMemory:
			mem = quota
		case corev1.ResourcePods:
			pods = quota
		case corev1.ResourceEphemeralStorage:
			empStorage = quota
		default:
			customResource[resourceName] = quota
		}
	}
	return &Resource{
		CPU:              cpu,
		Memory:           mem,
		Pods:             pods,
		EphemeralStorage: empStorage,
		Custom:           customResource,
	}
}
