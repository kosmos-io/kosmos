package api

import (
	"errors"
	"fmt"
)

// nolint:revive
const (
	ERR_NOT_FOUND     = "ErrNotFound"
	ERR_INVALID_INPUT = "ErrInvalidInput"
)

type causal interface {
	Cause() error
	error
}

type ErrNodeServer interface {
	GetErrorType() string
	error
}

type errNodeServer struct {
	errType string
	error
}

func (e *errNodeServer) GetErrorType() string {
	return e.errType
}

func ErrNotFound(msg string) error {
	return &errNodeServer{ERR_NOT_FOUND, errors.New(msg)}
}

func ErrInvalidInput(msg string) error {
	return &errNodeServer{ERR_INVALID_INPUT, errors.New(msg)}
}

func IsMatchErrType(err error, errType string) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrNodeServer); ok {
		return e.GetErrorType() == errType
	}

	if e, ok := err.(causal); ok {
		return IsMatchErrType(e.Cause(), errType)
	}

	return false
}

func IsNotFound(err error) bool {
	return IsMatchErrType(err, ERR_NOT_FOUND)
}

func IsInvalidInput(err error) bool {
	return IsMatchErrType(err, ERR_INVALID_INPUT)
}

func ConvertNotFound(err error) error {
	return &errNodeServer{ERR_NOT_FOUND, err}
}

func ConvertInvalidInput(err error) error {
	return &errNodeServer{ERR_INVALID_INPUT, err}
}

func ErrInvalidInputf(format string, args ...interface{}) error {
	return &errNodeServer{ERR_INVALID_INPUT, fmt.Errorf(format, args...)}
}
