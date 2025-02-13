package adaper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/projectcalico/calico/libcalico-go/lib/apiconfig"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/etcdv3"
	"github.com/projectcalico/calico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/calico/libcalico-go/lib/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockEtcdClient struct {
	mock.Mock
}

func (m *MockEtcdClient) Create(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	args := m.Called(ctx, object)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) Update(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	args := m.Called(ctx, object)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) Apply(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	args := m.Called(ctx, object)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	args := m.Called(ctx, key, revision)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) DeleteKVP(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	args := m.Called(ctx, object)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	args := m.Called(ctx, key, revision)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPair), args.Error(1)
}

func (m *MockEtcdClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	args := m.Called(ctx, list, revision)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.KVPairList), args.Error(1)
}

func (m *MockEtcdClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	args := m.Called(ctx, list, revision)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(api.WatchInterface), args.Error(1)
}

func (m *MockEtcdClient) EnsureInitialized() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockEtcdClient) Clean() error {
	args := m.Called()
	return args.Error(0)
}

func TestGetCIDRByNodeName(t *testing.T) {
	mockClient := new(MockEtcdClient)
	adapter := &CalicoETCDAdapter{etcdClient: mockClient}

	expectedCIDR := "10.1.1.0/26"
	mockBlockAffinityKey := model.BlockAffinityKey{
		CIDR: *mustParseCIDR(expectedCIDR),
		Host: "node1",
	}

	mockKVPair := &model.KVPair{
		Key: mockBlockAffinityKey,
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(&model.KVPairList{KVPairs: []*model.KVPair{mockKVPair}}, nil)

	tests := []struct {
		name        string
		nodeName    string
		expectCidrs []string
	}{
		{
			name:        "valid node",
			nodeName:    "node1",
			expectCidrs: []string{"10.1.1.0/26"},
		},
		{
			name:        "invalid node",
			nodeName:    "no-nodeName",
			expectCidrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cidrs, err := adapter.GetCIDRByNodeName(tt.nodeName)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectCidrs, cidrs)
		})
	}
	mockClient.AssertExpectations(t)
}

func mustParseCIDR(cidr string) *net.IPNet {
	_, parsedCIDR, _ := net.ParseCIDR(cidr)
	return parsedCIDR
}

func TestEtcdV3ClientWithMock(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		nodeName    string
		expectValue model.BlockAffinity
		expectErr   error
	}{
		{
			name:     "test1",
			cidr:     "10.1.1.0/26",
			nodeName: "node1",
			expectValue: model.BlockAffinity{
				State:   "confirmed",
				Deleted: false,
			},
			expectErr: nil,
		},
		{
			name:        "test2",
			cidr:        "10.1.1.0/26",
			nodeName:    "node2",
			expectValue: model.BlockAffinity{},
			expectErr:   errors.New("key not found"),
		},
	}

	mockEtcd := new(MockEtcdClient)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, cidr, err := net.ParseCIDR(test.cidr)
			if err != nil {
				t.Fatalf("Failed to parse CIDR: %v", err)
			}
			key := model.BlockAffinityKey{
				Host: test.nodeName,
				CIDR: *cidr,
			}

			expect := test.expectValue
			mockValue := &model.KVPair{
				Value: &expect,
			}
			mockEtcd.On("Get", ctx, key, "").Return(mockValue, test.expectErr)

			get, err := mockEtcd.Get(ctx, key, "")
			if err != nil {
				assert.Equal(t, test.expectErr.Error(), err.Error())
				return
			}

			if get != nil {
				value, ok := get.Value.(*model.BlockAffinity)
				if !ok {
					t.Errorf("Value is not BlockAffinity, err:%v\n", err)
				}
				assert.Equal(t, test.expectValue.State, value.State)
				assert.Equal(t, test.expectValue.Deleted, value.Deleted)
			}

			mockEtcd.AssertExpectations(t)
		})
	}
}

// nolint
func createEtcdClient() (api.Client, error) {
	etcdConfig := apiconfig.EtcdConfig{
		EtcdEndpoints:  "127.0.0.1:2379",
		EtcdKeyFile:    "/ssl/server.key",
		EtcdCertFile:   "/ssl/server.crt",
		EtcdCACertFile: "/ssl/ca.crt",
	}
	etcdV3Client, err := etcdv3.NewEtcdV3Client(&etcdConfig)
	if err != nil {
		return nil, err
	}
	return etcdV3Client, nil
}
