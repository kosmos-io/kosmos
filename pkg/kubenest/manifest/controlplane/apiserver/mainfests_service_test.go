package apiserver

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func ParseServerTemplate(apiServerServiceSubnet string) (*corev1.Service, error) {
	ipFamilies := utils.IPFamilyGenerator(apiServerServiceSubnet)
	apiserverServiceBytes, err := util.ParseTemplate(ApiserverService, struct {
		ServiceName, Namespace, ServiceType string
		ServicePort                         int32
		IPFamilies                          []corev1.IPFamily
	}{
		ServiceName: fmt.Sprintf("%s-%s", "test", "apiserver"),
		Namespace:   "test-namespace",
		ServiceType: constants.ApiServerServiceType,
		ServicePort: 40010,
		IPFamilies:  ipFamilies,
	})

	if err != nil {
		return nil, fmt.Errorf("error when parsing virtualClusterApiserver serive template: %s", err)
	}

	apiserverService := &corev1.Service{}
	if err := yaml.Unmarshal([]byte(apiserverServiceBytes), apiserverService); err != nil {
		return nil, fmt.Errorf("error when decoding virtual cluster apiserver service: %s", err)
	}
	return apiserverService, nil
}

func CompareIPFamilies(a []corev1.IPFamily, b []corev1.IPFamily) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestSyncIPPool(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []corev1.IPFamily
	}{
		{
			name:  "ipv4 only",
			input: "10.237.6.0/18",
			want:  []corev1.IPFamily{corev1.IPv4Protocol},
		},
		{
			name:  "ipv6 only",
			input: "2409:8c2f:3800:0011::0a18:0000/114",
			want:  []corev1.IPFamily{corev1.IPv6Protocol},
		},
		{
			name:  "ipv4 first",
			input: "10.237.6.0/18,2409:8c2f:3800:0011::0a18:0000/114",
			want:  []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
		},
		{
			name:  "ipv6 first",
			input: "2409:8c2f:3800:0011::0a18:0000/114,10.237.6.0/18",
			want:  []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := ParseServerTemplate(tt.input)
			if err != nil {
				t.Fatalf("happen error: %s", err)
			}

			if !CompareIPFamilies(svc.Spec.IPFamilies, tt.want) {
				t.Errorf("ParseServerTemplate()=%v, want %v", svc.Spec.IPFamilies, tt.want)
			}
		})
	}
}
