package types

// HandlerInitializer is a function that initializes and returns a new instance of one of action interfaces
type HandlerInitializer func() (interface{}, error)

type ItemKey struct {
	Resource  string
	Namespace string
	Name      string
}

type RestoredItemStatus struct {
	Action     string
	ItemExists bool
}
