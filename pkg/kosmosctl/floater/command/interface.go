package command

import "fmt"

const (
	ExecError = iota
	CommandSuccessed
	CommandFailed
)

type Result struct {
	Status    int
	ResultStr string
}

type Command interface {
	GetCommandStr() string
	ParseResult(string) *Result
}

func ParseError(err error) *Result {
	return &Result{
		Status:    ExecError,
		ResultStr: fmt.Sprintf("exec error: %s", err),
	}
}

func PrintStatus(status int) string {
	if status == ExecError {
		return "EXCEPTION"
	}
	if status == CommandSuccessed {
		return "SUCCESSED"
	}
	if status == CommandFailed {
		return "FAILED"
	}
	return "UNEXCEPTIONED"
}
