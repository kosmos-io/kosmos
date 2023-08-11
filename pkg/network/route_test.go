package network

import (
	"fmt"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

// go test . -run TestRoute
func TestRoute(t *testing.T) {
	t.Run("test loadRoute", func(t *testing.T) {

		device := &clusterlinkv1alpha1.Device{
			Type:    "vxlan",
			Name:    "vx-bridge",
			Addr:    "220.20.30.130/8", // some rule
			Mac:     "a6:02:63:1f:cd:a3",
			BindDev: "ens33",
			ID:      99,
			Port:    4578,
		}

		if err := deleteDevice(*device); err != nil {
			t.Error(err)
			return
		}

		route := &clusterlinkv1alpha1.Route{
			CIDR: "242.222.0.0/18",
			Gw:   "220.20.30.131", // next jump addr
			Dev:  "vx-bridge",
		}

		if err := addDevice(*device); err != nil {
			t.Error(err)
			return
		}

		if err := addRoute(*route); err != nil {
			t.Error(err)
			return
		}

		routes, err := loadRoutes()
		if err != nil {
			t.Error(err)
			return
		}

		if len(routes) > 1 {
			t.Error(fmt.Errorf("route len > 0 %s", routes))
			return
		}

		if route.Compare(routes[0]) == false {
			t.Error(fmt.Errorf("route is not match %s", routes))
			return
		}

		if err := deleteRoute(*route); err != nil {
			t.Error(err)
			return
		}

		if err := deleteDevice(*device); err != nil {
			t.Error(err)
			return
		}

	})
}
