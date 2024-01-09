package clusterlink

const (
	FLANNEL             = "flannel"
	TECENT_GLOBALROUTER = "globalrouter"
)

var (
	NonMasqCNI = map[string]struct{}{
		FLANNEL:             {},
		TECENT_GLOBALROUTER: {},
	}
	NonMasqCNISlice = []string{
		FLANNEL, TECENT_GLOBALROUTER,
	}
)
