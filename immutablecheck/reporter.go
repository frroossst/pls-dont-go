package immutablecheck

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

func getSourceLine(filename string, lineNum int) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer func() {
		if err := file.Close(); err != nil && err == nil {
		}
	}()

	scanner := bufio.NewScanner(file)
	currentLine := 1
	for scanner.Scan() {
		if currentLine == lineNum {
			return scanner.Text()
		}
		currentLine++
	}
	return ""
}

func getExpressionString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return getExpressionString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		return getExpressionString(e.X) + "[...]"
	case *ast.StarExpr:
		return "*" + getExpressionString(e.X)
	case *ast.ParenExpr:
		return "(" + getExpressionString(e.X) + ")"
	case *ast.UnaryExpr:
		return e.Op.String() + getExpressionString(e.X)
	case *ast.CallExpr:
		return getExpressionString(e.Fun) + "(...)"
	default:
		return "<expression>"
	}
}

func reportMutation(pass *analysis.Pass, pos token.Pos, exprStr string, expr ast.Expr, immutableTypes map[string]immutableInfo, helpMsg string) {
	typeName := getImmutableTypeName(pass, expr, immutableTypes)

	position := pass.Fset.Position(pos)
	sourceLine := getSourceLine(position.Filename, position.Line)

	if typeName == "" {
		msg := formatError(position, exprStr, "", position, sourceLine, helpMsg)
		pass.Reportf(pos, "%s", msg)
		return
	}

	info, exists := immutableTypes[typeName]
	if !exists {
		msg := formatError(position, exprStr, typeName, position, sourceLine, helpMsg)
		pass.Reportf(pos, "%s", msg)
		return
	}

	declPosition := pass.Fset.Position(info.pos)

	msg := formatError(position, exprStr, typeName, declPosition, sourceLine, helpMsg)
	pass.Reportf(pos, "%s", msg)
}

func formatError(mutationPos token.Position, exprStr string, typeName string, declPos token.Position, sourceLine string, helpMsg string) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("error: cannot mutate immutable type")
	sb.WriteString("\n")

	relPath := filepath.Base(mutationPos.Filename)
	sb.WriteString(fmt.Sprintf("  --> %s:%d:%d\n", relPath, mutationPos.Line, mutationPos.Column))

	if sourceLine != "" {
		sb.WriteString("   |\n")
		sb.WriteString(fmt.Sprintf("%4d | %s\n", mutationPos.Line, sourceLine))
		sb.WriteString("   |\n")
	}

	if typeName != "" {
		sb.WriteString(fmt.Sprintf("   = note: '%s' is a field of immutable type '%s'\n", exprStr, typeName))

		declRelPath := filepath.Base(declPos.Filename)
		sb.WriteString(fmt.Sprintf("   = note: '%s' was marked @immutable at %s:%d:%d\n",
			typeName, declRelPath, declPos.Line, declPos.Column))
	} else {
		sb.WriteString(fmt.Sprintf("   = note: attempting to mutate '%s'\n", exprStr))
	}

	// Always show the @allow-mutate suppression note
	sb.WriteString("   = note: use //@allow-mutate comment inline to suppress this error if needed\n")

	return sb.String()
}
