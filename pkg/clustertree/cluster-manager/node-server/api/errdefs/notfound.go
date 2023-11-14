// nolint:dupl
package errdefs

import (
	"errors"
	"fmt"
)

type ErrNotFound interface {
	NotFound() bool
	error
}

type notFoundError struct {
	error
}

func (e *notFoundError) NotFound() bool {
	return true
}

func (e *notFoundError) Cause() error {
	return e.error
}

func AsNotFound(err error) error {
	if err == nil {
		return nil
	}
	return &notFoundError{err}
}

func NotFound(msg string) error {
	return &notFoundError{errors.New(msg)}
}

func NotFoundf(format string, args ...interface{}) error {
	return &notFoundError{fmt.Errorf(format, args...)}
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrNotFound); ok {
		return e.NotFound()
	}

	if e, ok := err.(causal); ok {
		return IsNotFound(e.Cause())
	}

	return false
}
