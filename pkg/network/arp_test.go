package network

// func addIpt
import (
	"fmt"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func TestArps(t *testing.T) {
	t.Run("test create and load arp", func(t *testing.T) {

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

		record := &clusterlinkv1alpha1.Arp{
			IP:  "220.76.1.4",
			Mac: "a6:01:63:1f:cd:a2",
			Dev: "vx-bridge",
		}

		if err := addArp(*record); err != nil {
			t.Error(err)
			return
		}

		records, err := loadArps()

		if err != nil {
			t.Error(err)
			return
		}
		if len(records) > 1 {
			t.Error(fmt.Errorf("len > 0 %s", records))
			return
		}
		if !record.Compare(records[0]) {
			t.Error(fmt.Errorf("not match %s", records))
			return
		}

		if err := deleteArp(*record); err != nil {
			t.Error(err)
			return
		}

		deletes, delerr := loadArps()

		if delerr != nil {
			t.Error(delerr)
			return
		}

		if len(deletes) > 0 {
			t.Error(fmt.Errorf("delete failed %s", deletes))
			return
		}

		if err := deleteDevice(*device); err != nil {
			t.Error(err)
			return
		}

	})
}
