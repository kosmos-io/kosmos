package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"github.com/kosmos.io/kosmos/hack/projectpath"
)

func main() {
	clusterNodeCRD, err := os.ReadFile(fmt.Sprintf("%s/deploy/crds/kosmos.io_clusternodes.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	clusterCRD, err := os.ReadFile(fmt.Sprintf("%s/deploy/crds/kosmos.io_clusters.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	nodeConfigCRD, err := os.ReadFile(fmt.Sprintf("%s/deploy/crds/kosmos.io_nodeconfigs.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	serviceImportCRD, err := os.ReadFile(fmt.Sprintf("%s/deploy/crds/mcs/multicluster.x-k8s.io_serviceimports.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	serviceExportCRD, err := os.ReadFile(fmt.Sprintf("%s/deploy/crds/mcs/multicluster.x-k8s.io_serviceexports.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}

	filename := fmt.Sprintf("%s/pkg/kosmosctl/manifest/manifest_crds.go", projectpath.Root)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == "ClusterNode" {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", clusterNodeCRD),
			}
			return false
		}
		return true
	})

	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == "Cluster" {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", clusterCRD),
			}
			return false
		}
		return true
	})

	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == "NodeConfig" {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", nodeConfigCRD),
			}
			return false
		}
		return true
	})

	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == "ServiceImport" {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", serviceImportCRD),
			}
			return false
		}
		return true
	})

	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == "ServiceExport" {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", serviceExportCRD),
			}
			return false
		}
		return true
	})

	var buf bytes.Buffer
	err = format.Node(&buf, fset, node)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	code := buf.String()
	//fmt.Println(code)

	err = os.WriteFile(filename, []byte(code), 0600)
	if err != nil {
		fmt.Println("update failure", err)
		return
	}
}
