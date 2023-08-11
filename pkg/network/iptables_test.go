package network

// func addIpt
import (
	"fmt"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func TestIptables(t *testing.T) {
	t.Run("test create and load iptables", func(t *testing.T) {

		record := &clusterlinkv1alpha1.Iptables{
			Table: "nat",
			Chain: "PREROUTING",
			Rule:  "-d 242.222.0.0/18 -j NETMAP --to 10.222.0.0/18",
		}

		if err := addIptables(*record); err != nil {
			t.Error(err)
			return
		}

		records, err := loadIptables()

		if err != nil {
			t.Error(err)
			return
		}
		fmt.Println(records)
		if len(records) > 1 {
			t.Error(fmt.Errorf("len > 0 %s", records))
			return
		}
		if !record.Compare(records[0]) {
			t.Error(fmt.Errorf("not match %s", records))
			return
		}

		if err := deleteIptable(*record); err != nil {
			t.Error(err)
			return
		}

		deletes, delerr := loadIptables()

		if delerr != nil {
			t.Error(delerr)
			return
		}

		if len(deletes) > 0 {
			t.Error(fmt.Errorf("delete failed %s", deletes))
			return
		}

	})
}
