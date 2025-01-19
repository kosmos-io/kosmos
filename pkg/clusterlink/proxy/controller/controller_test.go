package controller

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynfake "k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	fakekosmosclient "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned/fake"
	informerfactory "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
)

var apiGroupResources = []*restmapper.APIGroupResources{
	{
		Group: metav1.APIGroup{
			Name: "apps",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "apps/v1", Version: "v1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "apps/v1", Version: "v1",
			},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {
				{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment"},
			},
		},
	},
	{
		Group: metav1.APIGroup{
			Name: "",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "v1", Version: "v1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "v1", Version: "v1",
			},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {
				{Name: "pods", SingularName: "pod", Namespaced: true, Kind: "Pod"},
			},
		},
	},
}

func TestNewResourceCacheController(t *testing.T) {
	type args struct {
		option NewControllerOption
	}
	dyClient, _ := dynfake.NewForConfig(&rest.Config{})
	o := NewControllerOption{
		DynamicClient: dyClient,
		KosmosFactory: informerfactory.NewSharedInformerFactory(fakekosmosclient.NewSimpleClientset(), 0),
		RestConfig:    &rest.Config{},
		RestMapper:    restmapper.NewDiscoveryRESTMapper(apiGroupResources),
	}
	tests := []struct {
		name    string
		args    args
		want    *ResourceCacheController
		wantErr bool
		errMsg  string
	}{
		{
			name: "NewResourceCacheController",
			args: args{
				option: o,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewResourceCacheController(tt.args.option)
			if err == nil && tt.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error message %s to be in %s", tt.errMsg, err.Error())
			}
		})
	}
}
