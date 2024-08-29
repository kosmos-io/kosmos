package controller

import (
	"fmt"
	"testing"
)

func TestNetxFunc(t *testing.T) {
	portsPool := []int32{1, 2, 3, 4, 5}
	type nextfunc func() (int32, error)
	// var next nextfunc
	next := func() nextfunc {
		i := 0
		return func() (int32, error) {
			if i >= len(portsPool) {
				return 0, fmt.Errorf("no available ports")
			}
			port := portsPool[i]
			i++
			return port, nil
		}
	}()

	for p, err := next(); err == nil; p, err = next() {
		fmt.Printf("port: %d\n", p)
	}
}

func TestCreateApiAnpServer(t *testing.T) {
	var name, namespace string
	apiAnpAgentSvc := createApiAnpAgentSvc(name, namespace, nameMap)

	if len(apiAnpAgentSvc.Spec.Ports) != 4 {
		t.Fatalf("apiAnpAgentSvc.Spec.Ports len != 4")
	}
	ports := make([]int32, 5)
	for _, port := range apiAnpAgentSvc.Spec.Ports {
		v, ok := nameMap[port.Name]
		if ok {
			ports[v] = port.Port
		} else {
			t.Fatalf("can not get node port for %s", port.Name)
		}
	}

	if ports[1] != 8081 {
		t.Fatalf("apiAnpAgentSvc.Spec.Ports[0].Port != 8081")
	}

	if ports[2] != 8082 {
		t.Fatalf("apiAnpAgentSvc.Spec.Ports[1].Port != 8082")
	}

	if ports[3] != 8083 {
		t.Fatalf("apiAnpAgentSvc.Spec.Ports[2].Port != 8083")
	}

	if ports[4] != 8084 {
		t.Fatalf("apiAnpAgentSvc.Spec.Ports[2].Port != 8084")
	}
}
