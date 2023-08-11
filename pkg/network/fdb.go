package network

import clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"

type FDBRecord struct {
	IP    string
	Mac   string
	Dev   string
	DevIP string
}

func (r *FDBRecord) IsEqual(fdb FDBRecord) bool {
	return r.Dev == fdb.Dev && r.Mac == fdb.Mac && r.IP == fdb.IP
}

func addFDB(ip string, mac string, devName string) error {
	return AddNeigh(ip, mac, NEIGH_FDB, devName)
}

func deleteFDB(ip string, mac string, devName string) error {
	return DeleteNeigh(ip, mac, NEIGH_FDB, devName)
}

func DeleteFDBByDevice(dev string) error {
	return DeleteNeighByDevice(dev, NEIGH_FDB)
}

func ListFDB() []FDBRecord {
	return ListNeigh(NEIGH_FDB)
}

func loadFdbs() ([]clusterlinkv1alpha1.Fdb, error) {
	ret := []clusterlinkv1alpha1.Fdb{}

	fdbs := ListFDB()
	for _, fdb := range fdbs {
		ret = append(ret, clusterlinkv1alpha1.Fdb{
			IP:  fdb.IP,
			Mac: fdb.Mac,
			Dev: fdb.Dev,
		})
	}
	return ret, nil
}

func deleteFdb(fdb clusterlinkv1alpha1.Fdb) error {
	return deleteFDB(fdb.IP, fdb.Mac, fdb.Dev)
}

func addFdb(fdb clusterlinkv1alpha1.Fdb) error {
	return addFDB(fdb.IP, fdb.Mac, fdb.Dev)
}
