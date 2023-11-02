package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/kosmos.io/kosmos/hack/projectpath"
)

func readFileAndTransformBackQuote(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(content), "`")
	return strings.Join(parts, "` + \"`\" + `"), nil
}

func updateAST(node *ast.File, name string, value string) {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Con && ident.Obj.Name == name {
			valueSpec := ident.Obj.Decl.(*ast.ValueSpec)
			valueSpec.Values[0] = &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("`%s`", value),
			}
			found = true
			return false
		}
		return true
	})

	if !found {
		// Add new variable if not found
		valueSpec := &ast.ValueSpec{
			Names: []*ast.Ident{ast.NewIdent(name)},
			Values: []ast.Expr{
				&ast.BasicLit{
					Kind:  token.STRING,
					Value: fmt.Sprintf("`%s`", value),
				},
			},
		}
		decl := &ast.GenDecl{
			Tok:   token.CONST,
			Specs: []ast.Spec{valueSpec},
		}
		node.Decls = append(node.Decls, decl)
	}
}

func main() {
	clusterNodeCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/kosmos.io_clusternodes.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	clusterCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/kosmos.io_clusters.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	nodeConfigCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/kosmos.io_nodeconfigs.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	serviceImportCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/mcs/multicluster.x-k8s.io_serviceimports.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}
	serviceExportCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/mcs/multicluster.x-k8s.io_serviceexports.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}

	daemonSetCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/kosmos.io_daemonsets.yaml", projectpath.Root))
	if err != nil {
		fmt.Println("can not read file：", err)
		return
	}

	shadowDaemonSetCRD, err := readFileAndTransformBackQuote(fmt.Sprintf("%s/deploy/crds/kosmos.io_shadowdaemonsets.yaml", projectpath.Root))
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

	updateAST(node, "ClusterNode", clusterNodeCRD)
	updateAST(node, "Cluster", clusterCRD)
	updateAST(node, "NodeConfig", nodeConfigCRD)
	updateAST(node, "ServiceImport", serviceImportCRD)
	updateAST(node, "ServiceExport", serviceExportCRD)
	updateAST(node, "DaemonSet", daemonSetCRD)
	updateAST(node, "ShadowDaemonSet", shadowDaemonSetCRD)

	var buf bytes.Buffer
	err = format.Node(&buf, fset, node)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	code := buf.String()

	err = os.WriteFile(filename, []byte(code), 0600)
	if err != nil {
		fmt.Println("update failure", err)
		return
	}
}
