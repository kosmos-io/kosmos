//go:build tools
//+build tools

package tools

import (
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
)
