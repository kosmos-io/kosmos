package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dyfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	v1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate"
	proxytest "github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/testing"
	fakekosmosclient "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned/fake"
	kosmosInformer "github.com/kosmos.io/kosmos/pkg/generated/informers/externalversions"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func TestNewResourceCacheController(t *testing.T) {
	restConfig := &rest.Config{
		Host: "https://localhost:6443",
	}
	rc := &v1alpha1.ResourceCache{
		ObjectMeta: metav1.ObjectMeta{Name: "rc"},
		Spec: v1alpha1.ResourceCacheSpec{
			ResourceCacheSelectors: []v1alpha1.ResourceCacheSelector{
				proxytest.PodResourceCacheSelector,
			},
		},
	}
	kosmosFactory := kosmosInformer.NewSharedInformerFactory(fakekosmosclient.NewSimpleClientset(rc), 0)
	o := NewControllerOption{
		DynamicClient: dyfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			proxytest.PodGVR: "PodList",
		}),
		KosmosFactory: kosmosFactory,
		RestConfig:    restConfig,
		RestMapper:    proxytest.RestMapper,
	}
	proxyCtl, err := NewResourceCacheController(o)
	if err != nil {
		t.Error(err)
		return
	}
	if proxyCtl == nil {
		t.Error("proxyCtl is nil")
		return
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	kosmosFactory.Start(stopCh)
	// start proxyctl
	go func() {
		proxyCtl.Run(stopCh, 1)
		defer proxyCtl.Stop()
	}()
	kosmosFactory.WaitForCacheSync(stopCh)
	time.Sleep(time.Second)
	hasPod := proxyCtl.store.HasResource(proxytest.PodGVR)
	if !hasPod {
		t.Error("has no pod resource cached")
		return
	}
}

func TestResourceCacheController_syncResourceCache(t *testing.T) {
	newMultiNs := func(namespaces ...string) *utils.MultiNamespace {
		multiNs := utils.NewMultiNamespace()
		if len(namespaces) == 0 {
			multiNs.Add(metav1.NamespaceAll)
			return multiNs
		}
		for _, ns := range namespaces {
			multiNs.Add(ns)
		}
		return multiNs
	}

	tests := []struct {
		name  string
		input []runtime.Object
		want  map[string]*utils.MultiNamespace
	}{
		{
			name: "cache pod resource with two namespace",
			input: []runtime.Object{
				&v1alpha1.ResourceCache{
					ObjectMeta: metav1.ObjectMeta{Name: "rc1"},
					Spec: v1alpha1.ResourceCacheSpec{
						ResourceCacheSelectors: []v1alpha1.ResourceCacheSelector{
							proxytest.PodResourceCacheSelector,
						},
					},
				},
			},
			want: map[string]*utils.MultiNamespace{
				"pods": newMultiNs("ns1", "ns2"),
			},
		},
		{
			name: "cache pod twice in two ResourceCache with different namespace",
			input: []runtime.Object{
				&v1alpha1.ResourceCache{
					ObjectMeta: metav1.ObjectMeta{Name: "rc1"},
					Spec: v1alpha1.ResourceCacheSpec{
						ResourceCacheSelectors: []v1alpha1.ResourceCacheSelector{
							proxytest.PodSelectorWithNS1,
						},
					},
				},
				&v1alpha1.ResourceCache{
					ObjectMeta: metav1.ObjectMeta{Name: "rc2"},
					Spec: v1alpha1.ResourceCacheSpec{
						ResourceCacheSelectors: []v1alpha1.ResourceCacheSelector{
							proxytest.PodSelectorWithNS2,
						},
					},
				},
			},
			want: map[string]*utils.MultiNamespace{
				"pods": newMultiNs("ns1", "ns2"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := map[string]*utils.MultiNamespace{}
			kosmosClientSet := fakekosmosclient.NewSimpleClientset(tt.input...)
			kosmosFactory := kosmosInformer.NewSharedInformerFactory(kosmosClientSet, 0)
			ctl := &ResourceCacheController{
				restMapper:          proxytest.RestMapper,
				resourceCacheLister: kosmosFactory.Kosmos().V1alpha1().ResourceCaches().Lister(),
				store: &proxytest.MockStore{
					UpdateCacheFunc: func(resources map[schema.GroupVersionResource]*utils.MultiNamespace) error {
						for k, v := range resources {
							actual[k.Resource] = v
						}
						return nil
					},
				},
			}
			stopCh := make(chan struct{})
			kosmosFactory.Start(stopCh)
			kosmosFactory.WaitForCacheSync(stopCh)
			err := ctl.syncResourceCache("test")
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("diff: %v", cmp.Diff(actual, tt.want))
			}
		})
	}
}

func TestResourceCacheController_Connect(t *testing.T) {
	store := &proxytest.MockStore{
		HasResourceFunc: func(gvr schema.GroupVersionResource) bool { return gvr == proxytest.PodGVR },
	}
	tests := []struct {
		name       string
		plugins    []*proxytest.MockDelegate
		wantErr    bool
		wantCalled []bool
	}{
		{
			name: "call first",
			plugins: []*proxytest.MockDelegate{
				{
					MockOrder:        0,
					IsSupportRequest: true,
				},
				{
					MockOrder:        1,
					IsSupportRequest: true,
				},
			},
			wantErr:    false,
			wantCalled: []bool{true, false},
		},
		{
			name: "call second",
			plugins: []*proxytest.MockDelegate{
				{
					MockOrder:        0,
					IsSupportRequest: false,
				},
				{
					MockOrder:        1,
					IsSupportRequest: true,
				},
			},
			wantErr:    false,
			wantCalled: []bool{false, true},
		},
		{
			name: "call fail",
			plugins: []*proxytest.MockDelegate{
				{
					MockOrder:        0,
					IsSupportRequest: false,
				},
				{
					MockOrder:        1,
					IsSupportRequest: false,
				},
			},
			wantErr:    true,
			wantCalled: []bool{false, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctl := &ResourceCacheController{
				delegate:             delegate.NewDelegateChain(proxytest.ConvertPluginSlice(tt.plugins)),
				negotiatedSerializer: scheme.Codecs.WithoutConversion(),
				store:                store,
			}

			conn, err := ctl.Connect(context.TODO(), "/api/v1/pods", nil)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest(http.MethodGet, "/prefix/api/v1/pods", nil)
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()
			conn.ServeHTTP(recorder, req)

			response := recorder.Result()

			if (response.StatusCode != 200) != tt.wantErr {
				t.Errorf("http request returned status code = %v, want error = %v",
					response.StatusCode, tt.wantErr)
			}

			if len(tt.plugins) != len(tt.wantCalled) {
				panic("len(tt.plugins) != len(tt.wantCalled), please fix test cases")
			}

			for i, n := 0, len(tt.plugins); i < n; i++ {
				if tt.plugins[i].Called != tt.wantCalled[i] {
					t.Errorf("plugin[%v].Called = %v, want = %v", i, tt.plugins[i].Called, tt.wantCalled[i])
				}
			}
		})
	}
}
