package cluster

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func prepareData(crds string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
					Command: []string{
						"kube-apiserver",
						fmt.Sprintf("--service-cluster-ip-range=%s", crds),
						"--profiling=false",
					},
				},
			},
		},
	}
}

func TestResolveServiceCIDRs(t *testing.T) {
	tests := []struct {
		name  string
		input *corev1.Pod
		want  []string
	}{
		{
			name:  "test ipv4 and ipv6",
			input: prepareData("2409:8c2f:3800:0011::0a18:0000/114,10.237.6.0/18"),
			want: []string{
				"2409:8c2f:3800:11::a18:0/114",
				"10.237.0.0/18",
			},
		},
		{
			name:  "test ipv4",
			input: prepareData("10.237.6.0/18"),
			want: []string{
				"10.237.0.0/18",
			},
		},
		{
			name:  "test ipv6",
			input: prepareData("2409:8c2f:3800:0011::0a18:0000/114"),
			want: []string{
				"2409:8c2f:3800:11::a18:0/114",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := ResolveServiceCIDRs(tt.input)
			if err != nil {
				t.Fatalf("ResolveServiceCIDRs err: %s", err.Error())
			}

			if strings.Join(ret, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("value is incorretc!")
			}
		})
	}
}
