// nolint:dupl
package errdefs

import (
	"errors"
	"fmt"
)

type ErrInvalidInput interface {
	InvalidInput() bool
	error
}

type invalidInputError struct {
	error
}

func (e *invalidInputError) InvalidInput() bool {
	return true
}

func (e *invalidInputError) Cause() error {
	return e.error
}

func ConvertToInvalidInput(err error) error {
	if err == nil {
		return nil
	}
	return &invalidInputError{err}
}

func InvalidInput(msg string) error {
	return &invalidInputError{errors.New(msg)}
}

func InvalidInputf(format string, args ...interface{}) error {
	return &invalidInputError{fmt.Errorf(format, args...)}
}

func IsInvalidInput(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrInvalidInput); ok {
		return e.InvalidInput()
	}

	if e, ok := err.(causal); ok {
		return IsInvalidInput(e.Cause())
	}

	return false
}
