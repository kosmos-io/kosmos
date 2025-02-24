package testing

import (
	"context"
	"net/http"

	"github.com/kosmos.io/kosmos/pkg/clusterlink/proxy/delegate"
)

type MockDelegate struct {
	MockOrder        int
	IsSupportRequest bool
	Called           bool
}

// Connect implements delegate.Delegate.
func (m *MockDelegate) Connect(_ context.Context, _ delegate.ProxyRequest) (http.Handler, error) {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		m.Called = true
	}), nil
}

// Order implements delegate.Delegate.
func (m *MockDelegate) Order() int {
	return m.MockOrder
}

// SupportRequest implements delegate.Delegate.
func (m *MockDelegate) SupportRequest(_ delegate.ProxyRequest) bool {
	return m.IsSupportRequest
}

var _ delegate.Delegate = (*MockDelegate)(nil)

func ConvertPluginSlice(in []*MockDelegate) []delegate.Delegate {
	out := make([]delegate.Delegate, 0, len(in))
	for _, plugin := range in {
		out = append(out, plugin)
	}

	return out
}
