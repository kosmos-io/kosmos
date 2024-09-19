package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindFirstNodeIPAddress(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.0.1",
				},
				{
					Type:    corev1.NodeExternalIP,
					Address: "192.168.0.2",
				},
				{
					Type:    corev1.NodeInternalDNS,
					Address: "192.168.0.3",
				},
				{
					Type:    corev1.NodeHostName,
					Address: "192.168.0.4",
				},
				{
					Type:    corev1.NodeExternalDNS,
					Address: "192.168.0.5",
				},
			},
		},
	}

	tests := []struct {
		name            string
		input           *corev1.Node
		want            string
		nodeAddressType corev1.NodeAddressType
	}{
		{
			string(corev1.NodeInternalIP),
			&node,
			"192.168.0.1",
			corev1.NodeInternalIP,
		},
		{
			string(corev1.NodeExternalIP),
			&node,
			"192.168.0.2",
			corev1.NodeExternalIP,
		},
		{
			string(corev1.NodeInternalDNS),
			&node,
			"192.168.0.3",
			corev1.NodeInternalDNS,
		},
		{
			string(corev1.NodeHostName),
			&node,
			"192.168.0.4",
			corev1.NodeHostName,
		},
		{
			string(corev1.NodeExternalDNS),
			&node,
			"192.168.0.5",
			corev1.NodeExternalDNS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := FindFirstNodeIPAddress(*tt.input, tt.nodeAddressType)
			if err != nil {
				t.Fatalf("%s, %s, %s", tt.name, tt.input, err)
			}
			if ip != tt.want {
				t.Fatalf("%s, %s, %s, %s", tt.name, tt.input, ip, tt.want)
			}
		})
	}
}
