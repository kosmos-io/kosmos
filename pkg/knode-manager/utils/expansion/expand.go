package expansion

import (
	"bytes"
	"strings"
)

const (
	operator       = '$'
	operatorOpener = '('
	operatorCloser = ')'
)

func syntaxWrap(input string) string {
	return strings.Join([]string{string(operator), string(operatorOpener), input, string(operatorCloser)}, "")
}

func MappingFuncFor(context ...map[string]string) func(string) string {
	return func(input string) string {
		for _, vars := range context {
			val, ok := vars[input]
			if ok {
				return val
			}
		}

		return syntaxWrap(input)
	}
}

func Expand(input string, mapping func(string) string) string {
	var buf bytes.Buffer
	checkpoint := 0
	for cursor := 0; cursor < len(input); cursor++ {
		if input[cursor] == operator && cursor+1 < len(input) {
			buf.WriteString(input[checkpoint:cursor])

			read, isVar, advance := tryReadVariableName(input[cursor+1:])

			if isVar {
				buf.WriteString(mapping(read))
			} else {
				buf.WriteString(read)
			}

			cursor += advance

			checkpoint = cursor + 1
		}
	}

	return buf.String() + input[checkpoint:]
}

func tryReadVariableName(input string) (string, bool, int) {
	switch input[0] {
	case operator:
		return input[0:1], false, 1
	case operatorOpener:
		for i := 1; i < len(input); i++ {
			if input[i] == operatorCloser {
				return input[1:i], true, i + 1
			}
		}
		return string(operator) + string(operatorOpener), false, 1
	default:
		return (string(operator) + string(input[0])), false, 1
	}
}
