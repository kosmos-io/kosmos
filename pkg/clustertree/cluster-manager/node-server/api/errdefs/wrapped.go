package errdefs

type causal interface {
	Cause() error
	error
}
