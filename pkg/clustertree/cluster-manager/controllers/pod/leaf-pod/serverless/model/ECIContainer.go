package model

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const KosmosServerlessAppendLabel = "kosmos.io/serverless.append"

type ECIContainer struct {
	Name             string     `json:"name,omitempty"`
	Region           string     `json:"region,omitempty"`
	Cpu              float64    `json:"cpu,omitempty"`
	Memory           float64    `json:"memory,omitempty"`
	Quantity         int32      `json:"quantity,omitempty"`
	Volumes          []Volume   `json:"volumes,omitempty"`
	VpcId            string     `json:"vpcId,omitempty"`
	NetworkId        string     `json:"networkId,omitempty"`
	SecurityGroupIds []string   `json:"securityGroupIds,omitempty"`
	IpId             string     `json:"ipId,omitempty"`
	IpType           string     `json:"ipType,omitempty"`
	ChargeMode       string     `json:"chargeMode,omitempty"`
	BandwidthSize    int32      `json:"bandwidthSize,omitempty"`
	Ipv4Bandwidth    bool       `json:"ipv4Bandwidth,omitempty"`
	Ipv6Bandwidth    bool       `json:"ipv6Bandwidth,omitempty"`
	Pod              corev1.Pod `json:"pod,omitempty"`
	EciId            string     `json:"eciId,omitempty"`

	// // get
	// Labels    map[string]string `json:"labels,omitempty"`
	// Status    string            `json:"status,omitempty"`
	// Namespace string            `json:"namespace,omitempty"`
}

type ECIContainerGet struct {
	Name             string     `json:"name,omitempty"`
	Region           string     `json:"region,omitempty"`
	Cpu              float64    `json:"cpu,omitempty"`
	Memory           float64    `json:"memory,omitempty"`
	Quantity         int32      `json:"quantity,omitempty"`
	Volumes          []Volume   `json:"volumes,omitempty"`
	VpcId            string     `json:"vpcId,omitempty"`
	NetworkId        string     `json:"networkId,omitempty"`
	SecurityGroupIds []string   `json:"securityGroupIds,omitempty"`
	IpId             string     `json:"ipId,omitempty"`
	IpType           string     `json:"ipType,omitempty"`
	ChargeMode       string     `json:"chargeMode,omitempty"`
	BandwidthSize    int32      `json:"bandwidthSize,omitempty"`
	Ipv4Bandwidth    bool       `json:"ipv4Bandwidth,omitempty"`
	Ipv6Bandwidth    bool       `json:"ipv6Bandwidth,omitempty"`
	Pod              corev1.Pod `json:"pod,omitempty"`
	EciId            string     `json:"eciId,omitempty"`

	// get
	Labels    map[string]string `json:"labels,omitempty"`
	Status    string            `json:"status,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
}

func NewECIContainer(pod *corev1.Pod) (*ECIContainer, error) {
	c := &ECIContainer{}

	annotations := pod.Annotations
	if annotations == nil {
		return nil, fmt.Errorf("serverless pod annotations is nil")
	}
	if appendValue, ok := annotations[KosmosServerlessAppendLabel]; !ok {
		return nil, fmt.Errorf("serverless pod appendValue is nil")
	} else {
		if err := json.Unmarshal([]byte(appendValue), c); err != nil {
			return nil, fmt.Errorf("serverless pod %s", err)
		}

		c.Pod = *pod
	}
	// missing kind
	c.Pod.Kind = "Pod"
	// kosmos label
	if c.Pod.Labels == nil {
		c.Pod.Labels = make(map[string]string)
	}
	c.Pod.Labels[utils.KosmosPodLabel] = "true"
	// TODO: fit serverless pod
	return c, nil
}

func (c *ECIContainerGet) ToK8sPod() (*corev1.Pod, error) {
	pod := c.Pod.DeepCopy()

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	// to map
	var podMap map[string]interface{}
	err = json.Unmarshal(jsonData, &podMap)
	if err != nil {
		return nil, err
	}
	delete(podMap, "Pod")

	labelValue, err := json.Marshal(podMap)
	if err != nil {
		return nil, err
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[KosmosServerlessAppendLabel] = string(labelValue)
	pod.Labels = c.Labels

	pod.Name = c.Name
	pod.Status.Phase = corev1.PodPhase(c.Status)
	pod.Namespace = c.Namespace
	if pod.Namespace == "" {
		pod.Namespace = "default"
	}

	return pod, nil
}

type Volume struct {
	ResourceType string `json:"resourceType,omitempty"`
	Size         int32  `json:"size,omitempty"`
}

type ListResult struct {
	PageNo         int               `json:"pageNo,omitempty"`
	PageSize       int               `json:"pageSize,omitempty"`
	Total          int               `json:"total,omitempty"`
	NextToken      string            `json:"nextToken,omitempty"`
	RemainingCount float64           `json:"remainingCount,omitempty"`
	Content        []ECIContainerGet `json:"content,omitempty"`
}

type ECIContainerStatus struct {
	Id               int32            `json:"id,omitempty"`
	ContainerGroupId string           `json:"containerGroupId,omitempty"`
	Name             string           `json:"name,omitempty"`
	Namespace        string           `json:"namespace,omitempty"`
	Status           string           `json:"status,omitempty"`
	PodStatus        corev1.PodStatus `json:"podStatus,omitempty"`
}

type ListStatusResult struct {
	PageNo         int                  `json:"pageNo,omitempty"`
	PageSize       int                  `json:"pageSize,omitempty"`
	Total          int                  `json:"total,omitempty"`
	NextToken      string               `json:"nextToken,omitempty"`
	RemainingCount float64              `json:"remainingCount,omitempty"`
	Content        []ECIContainerStatus `json:"content,omitempty"`
}

type CreateResult struct {
	OrderId   string    `json:"orderId,omitempty"`
	OrderFree string    `json:"orderFree,omitempty"`
	Products  []Product `json:"products,omitempty"`
}

type Product struct {
	OrderExtId string `json:"orderExtId,omitempty"`
	InstanceId string `json:"instanceId,omitempty"`
	SeqGroupId string `json:"seqGroupId,omitempty"`
	StartDate  string `json:"startDate,omitempty"`
	EndDate    string `json:"endDate,omitempty"`
	SequenceId int32  `json:"sequenceId,omitempty"`
}

type DeleteResult bool
