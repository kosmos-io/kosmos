package leafnodevolumebinding

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	fake2 "k8s.io/kubernetes/pkg/scheduler/framework/fake"
	scheduling "k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumebinding"
)

func TestStateDataClone(t *testing.T) {
	// initialize a statedata object
	state := &stateData{
		skip:         false,
		boundClaims:  []*v1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "claim1"}}},
		claimsToBind: []*v1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "claim2"}}},
		allBound:     false,
	}

	// Clone stateData
	clonedState := state.Clone().(*stateData)

	// make sure the cloned data is correct
	// Use the assert Equal method of unittest.Test Case to compare whether state.skip and cloned State.skip are equal.
	// If not equal, an AssertionError is thrown with the message "skip should be the same in cloned state"
	assert.Equal(t, state.skip, clonedState.skip, "skip should be the same in cloned state")
	// Use the assert Equal method of unittest.Test Case to compare whether the lengths of state.bound Claims and cloned State.bound Claims are equal.
	// If not equal, an AssertionError is thrown with the message "boundClaims length should match"
	assert.Equal(t, len(state.boundClaims), len(clonedState.boundClaims), "boundClaims length should match")
	// Use the assert Equal method of unittest.Test Case to compare whether state.bound Claims[0].Name and cloned State.bound Claims[0].Name are equal.
	// If not equal, an AssertionError is thrown with the message "boundClaims names should be equal"
	assert.Equal(t, state.boundClaims[0].Name, clonedState.boundClaims[0].Name, "boundClaims names should be equal")
	// Use the assert Equal method of unittest.Test Case to compare whether the lengths of state.claims To Bind and cloned State.claims To Bind are equal.
	// If not equal, an AssertionError is thrown with the message "claimsToBind length should match"
	assert.Equal(t, len(state.claimsToBind), len(clonedState.claimsToBind), "claimsToBind length should match")
	// Use the assert Equal method of unittest.Test Case to compare whether state.claims To Bind[0].Name and cloned State.claims To Bind[0].Name are equal.
	// If not equal, an AssertionError is thrown with the message "claimsToBind names should be equal"
	assert.Equal(t, state.claimsToBind[0].Name, clonedState.claimsToBind[0].Name, "claimsToBind names should be equal")
	// Use the assert Equal method of unittest.Test Case to compare whether state.all Bound and cloned State.all Bound are equal.
	// If not equal, an AssertionError is thrown with the message "allBound should be the same in cloned state"
	assert.Equal(t, state.allBound, clonedState.allBound, "allBound should be the same in cloned state")
}

func TestVolumeBinding_Name(t *testing.T) {
	// create a volumebinding instance
	volumeBinding := &VolumeBinding{}

	// Check that the name returned by the Name method is correct
	assert.Equal(t, "LeafNodeVolumeBinding", volumeBinding.Name(), "plugin name should be 'LeafNodeVolumeBinding'")
}

func TestVolumeBinding_ImplementInterfaces(t *testing.T) {
	// create a volumebinding instance
	volumeBinding := &VolumeBinding{}

	// Verify whether VolumeBinding implements the PreFilterPlugin interface
	var _ framework.PreFilterPlugin = volumeBinding

	// Verify whether VolumeBinding implements the FilterPlugin interface
	var _ framework.FilterPlugin = volumeBinding

	// Verify whether VolumeBinding implements the ReservePlugin interface
	var _ framework.ReservePlugin = volumeBinding

	// Verify whether VolumeBinding implements the PreBindPlugin interface
	var _ framework.PreBindPlugin = volumeBinding
}

type FakePVCInterface struct {
	client kubernetes.Interface
}

func (f *FakePVCInterface) PersistentVolumeClaims(namespace string) corelisters.PersistentVolumeClaimNamespaceLister {
	return &FakePVCNamespaceLister{
		client:    f.client,
		namespace: namespace,
	}
}

func (f *FakePVCInterface) List(selector labels.Selector) ([]*v1.PersistentVolumeClaim, error) {
	// Simulate returning a list of PVCs, this should return all PVCs in the namespace
	pvcList := []*v1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc2",
			},
		},
	}
	return pvcList, nil
}

type FakePVCNamespaceLister struct {
	client    kubernetes.Interface
	namespace string
}

func (f *FakePVCNamespaceLister) Get(name string) (*v1.PersistentVolumeClaim, error) {
	// Return the mock PVC
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, nil
}

func (f *FakePVCNamespaceLister) List(selector labels.Selector) ([]*v1.PersistentVolumeClaim, error) {
	// Simulate returning a list of PVCs for the namespace
	pvcList := []*v1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc2",
			},
		},
	}
	return pvcList, nil
}

func TestVolumeBinding_podHasPVCs(t *testing.T) {
	tests := []struct {
		name          string
		pod           *v1.Pod
		VolumeBinding *VolumeBinding
		expectedBool  bool
		expectedErr   error
	}{
		{
			name: "pod has no PVC, pvc count is 0",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-no-pvc",
				},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{{}},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: false,
			expectedErr:  nil,
		},
		{
			name: "pod has no PVC, pvc count is 1",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-no-pvc",
				},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-a",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: false,
			expectedErr:  nil,
		},
		{
			name: "pod have pvc, pvc count is 0",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-have-pvc",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-test",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-test",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  errors.New("persistentvolumeclaim \"pvc-test\" not found"),
		},
		{
			name: "pod have pvc, pvc is found",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-pvc",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-found",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-found",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-found",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  nil,
		},
		{
			name: "pvc is Ephemeral, but namespace is not equal",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-a",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-a",
						VolumeSource: v1.VolumeSource{
							Ephemeral: &v1.EphemeralVolumeSource{
								VolumeClaimTemplate: &v1.PersistentVolumeClaimTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Name: "pvc-a",
									},
								},
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-a-pvc-a",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-b",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  errors.New("PVC /pod-a-pvc-a was not created for pod /pod-a (pod is not owner)"),
		},
		{
			name: "pvc is Ephemeral, but namespace is equal",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a",
					Namespace: "test",
					UID:       "a",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-a",
						VolumeSource: v1.VolumeSource{
							Ephemeral: &v1.EphemeralVolumeSource{
								VolumeClaimTemplate: &v1.PersistentVolumeClaimTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Name: "pvc-a",
									},
								},
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-a-pvc-a",
							Namespace: "test",
							UID:       "a",
							OwnerReferences: []metav1.OwnerReference{
								{
									UID:        "a",
									Controller: func(b bool) *bool { return &b }(true),
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-b",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  nil,
		},
		{
			name: "pvc is lost",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc",
						},
						Status: v1.PersistentVolumeClaimStatus{
							Phase: v1.ClaimLost,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-b",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  errors.New("persistentvolumeclaim \"pvc\" bound to non-existent persistentvolume \"\""),
		},
		{
			name: "pvc is DeletionTimestamp is not nil",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "pvc",
							DeletionTimestamp: func(time metav1.Time) *metav1.Time { return &time }(metav1.Now()),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-b",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedBool: true,
			expectedErr:  errors.New("persistentvolumeclaim \"pvc\" is being deleted"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute PreFilter
			bool, err := tt.VolumeBinding.podHasPVCs(tt.pod)

			// Validate result
			assert.Equal(t, tt.expectedBool, bool)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestPreFilter(t *testing.T) {
	tests := []struct {
		name           string
		pod            *v1.Pod
		VolumeBinding  *VolumeBinding
		expectedStatus *framework.Status
	}{
		{
			name: "pod has no PVCs",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-no-pvc",
				},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-a",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedStatus: nil,
		},
		{
			name: "pod have pvc",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-have-pvc",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-test",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-test",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-test",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedStatus: nil,
		},
		{
			name: "pod have pvc, but not found",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-have-not-found-pvc",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-not-found",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-not-found",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-a",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedStatus: framework.NewStatus(framework.UnschedulableAndUnresolvable, "persistentvolumeclaim \"pvc-not-found\" not found"),
		},
		{
			name: "pod have pvc, but pvc have two",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-a-pvc",
				},
				Spec: v1.PodSpec{Volumes: []v1.Volume{
					{
						Name: "pvc-a",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc-a",
							},
						},
					},
				}},
			},
			VolumeBinding: &VolumeBinding{
				PVCLister: fake2.PersistentVolumeClaimLister{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-a",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc-b",
						},
					},
				},
				Binder: &scheduling.FakeVolumeBinder{
					AssumeCalled: false,
					BindCalled:   false,
				},
			},
			expectedStatus: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cycleState := framework.NewCycleState()

			// Execute PreFilter
			_, status := tt.VolumeBinding.PreFilter(ctx, cycleState, tt.pod)

			// Validate status
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func GetPVCName(pvc *v1.PersistentVolumeClaim) string {
	return pvc.Namespace + "/" + pvc.Name
}
func TestGetPVCName(t *testing.T) {
	tests := []struct {
		name           string
		pvc            *v1.PersistentVolumeClaim
		expectedResult string
	}{
		{
			name: "test1",
			pvc: &v1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test1",
					Namespace:         "default",
					Labels:            map[string]string{},
					Annotations:       map[string]string{},
					UID:               "1234567890",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Finalizers:        []string{"kubernetes"},
				},
				Spec: v1.PersistentVolumeClaimSpec{},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
			expectedResult: "default/test1",
		},
		{
			name: "test1",
			pvc: &v1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test2",
					Namespace:         "kube-system",
					Labels:            map[string]string{},
					Annotations:       map[string]string{},
					UID:               "1234567890",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Finalizers:        []string{"kubernetes"},
				},
				Spec: v1.PersistentVolumeClaimSpec{},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
			expectedResult: "kube-system/test2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute PreFilter
			result := getPVCName(tt.pvc)

			// Validate status
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCheckNodeAffinity(t *testing.T) {
	tests := []struct {
		name       string
		pv         *v1.PersistentVolume
		nodeLabels map[string]string
		wantErr    bool
	}{
		{
			name: "NodeAffinity not specified",
			pv: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{},
			},
			wantErr: true,
		},
		{
			name: "No matching NodeSelectorTerms",
			pv: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					NodeAffinity: &v1.VolumeNodeAffinity{
						Required: &v1.NodeSelector{
							NodeSelectorTerms: []v1.NodeSelectorTerm{
								{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      "key",
											Operator: v1.NodeSelectorOpExists,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Matching NodeSelectorTerms",
			pv: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					NodeAffinity: &v1.VolumeNodeAffinity{
						Required: &v1.NodeSelector{
							NodeSelectorTerms: []v1.NodeSelectorTerm{
								{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      "key",
											Operator: v1.NodeSelectorOpIn,
											Values:   []string{"value"},
										},
									},
								},
							},
						},
					},
				},
			},
			nodeLabels: map[string]string{
				"key": "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNodeAffinity(tt.pv, tt.nodeLabels)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckNodeAffinity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestByPVCSize_Len(t *testing.T) {
	// test the length of the pvc list
	pvcList := byPVCSize{
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc2"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("2Gi"),
					},
				},
			},
		},
	}

	// verify len method
	assert.Equal(t, 2, pvcList.Len())
}

func TestByPVCSize_Swap(t *testing.T) {
	// test the swap method
	pvcList := byPVCSize{
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc2"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("2Gi"),
					},
				},
			},
		},
	}

	// verify the order before swap method
	assert.Equal(t, "pvc1", pvcList[0].Name)
	assert.Equal(t, "pvc2", pvcList[1].Name)

	// execute swap
	pvcList.Swap(0, 1)

	// verify the order after the swap method
	assert.Equal(t, "pvc2", pvcList[0].Name)
	assert.Equal(t, "pvc1", pvcList[1].Name)
}

func TestByPVCSize_Less(t *testing.T) {
	// test the less method
	pvcList := byPVCSize{
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("2Gi"),
					},
				},
			},
		},
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc2"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	// Verify Less method, should return true because "1Gi" < "2Gi"
	assert.True(t, pvcList.Less(1, 0))
}

func TestByPVCSize_Sort(t *testing.T) {
	// test sorting the pvc list
	pvcList := byPVCSize{
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("2Gi"),
					},
				},
			},
		},
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc2"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	// perform sorting
	sort.Sort(pvcList)

	// verify sorting results
	assert.Equal(t, "pvc2", pvcList[0].Name)
	assert.Equal(t, "pvc1", pvcList[1].Name)
}
