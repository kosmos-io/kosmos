package network

import (
	"fmt"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func TestDevice(t *testing.T) {
	t.Run("test create and load device", func(t *testing.T) {
		device := &clusterlinkv1alpha1.Device{
			// Type:    "vxlan",
			// Name:    "vx-bridge",
			// Addr:    "220.20.30.130/8", // some rule
			// Mac:     "a6:02:63:1f:cd:a3",
			// BindDev: "ens33",
			// ID:      "99",
			// Port:    "4578",
			Type:    "vxlan",
			Name:    "vx-local",
			Addr:    "210.20.30.130/8", // some rule
			Mac:     "a6:02:63:1f:cd:a3",
			BindDev: "ens33",
			ID:      55,
			Port:    4866,
		}

		if err := deleteDevice(*device); err != nil {
			t.Error(err)
			return
		}

		if err := addDevice(*device); err != nil {
			t.Error(err)
			return
		}

		devices, err := loadDevices()

		if err != nil {
			t.Error(err)
			return
		}

		if len(devices) != 1 {
			t.Error(fmt.Errorf("len > 0"))
			return
		}

		if device.Compare(devices[0]) == false {
			t.Error(device, devices)
			return
		}

		if err := deleteDevice(*device); err != nil {
			t.Error(err)
			return
		}
	})
}
