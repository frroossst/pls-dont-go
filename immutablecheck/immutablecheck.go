package immutablecheck

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name:     "immutablecheck",
	Doc:      "check for mutations of @immutable marked types",
	Run:      run,
	Requires: []*analysis.Analyzer{},
}

func New(conf any) ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

type immutableInfo struct {
	typeName string
	pos      token.Pos
}

// check for parser errors, if they exist, skip analysis
func isParserOk(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		foundBad := false
		ast.Inspect(file, func(n ast.Node) bool {
			if _, ok := n.(*ast.BadExpr); ok {
				foundBad = true
				return false
			}
			if _, ok := n.(*ast.BadDecl); ok {
				foundBad = true
				return false
			}
			if _, ok := n.(*ast.BadStmt); ok {
				foundBad = true
				return false
			}
			return true
		})
		if foundBad {
			return false, nil
		}
	}
	return true, nil
}

// passCollector orchestrates the multi-pass analysis for immutability checking
type passCollector struct {
	pass                  *analysis.Pass
	immutableTypes        map[string]immutableInfo
	varToTypeAlias        map[types.Object]string
	copiedVariables       map[types.Object]bool
	aliasToImmutableField map[types.Object]bool
}

func newPassCollector(pass *analysis.Pass) *passCollector {
	return &passCollector{
		pass:                  pass,
		immutableTypes:        make(map[string]immutableInfo),
		varToTypeAlias:        make(map[types.Object]string),
		copiedVariables:       make(map[types.Object]bool),
		aliasToImmutableField: make(map[types.Object]bool),
	}
}

func (pc *passCollector) firstPass() {
	pc.collectImmutableTypes()
}

func (pc *passCollector) secondPass() {
	pc.trackTypeAliasVariables()
}

func (pc *passCollector) thirdPass() {
	pc.trackCopiesAndAliases()
}

func (pc *passCollector) fourthPass() {
	pc.checkMutations()
}

// collectImmutableTypes finds all types marked with @immutable annotation
func (pc *passCollector) collectImmutableTypes() {
	putLog(info, "started collecting immutable types")

	for _, file := range pc.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				// check for type declaration with `@immutable` comment
				if node.Tok == token.TYPE && hasImmutableComment(node, file.Comments) {
					for _, spec := range node.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							typeName := typeSpec.Name.Name
							pc.immutableTypes[typeName] = immutableInfo{
								typeName: typeName,
								pos:      typeSpec.Pos(),
							}
						}
					}
				}
			}
			return true
		})
	}

	putLog(info, "finished collecting immutable types")
	putLog(dbug, Pretty_print_immutables(&pc.immutableTypes))
}

// trackTypeAliasVariables tracks variables declared with immutable type aliases
func (pc *passCollector) trackTypeAliasVariables() {
	putLog(info, "started tracking type alias variables")

	for _, file := range pc.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				pc.processValueSpec(node)
			case *ast.AssignStmt:
				pc.processAssignStmt(node)
			}
			return true
		})
	}

	putLog(info, "finished tracking type alias variables")
}

// processValueSpec handles variable declarations like: var x AliasString = "hello"
func (pc *passCollector) processValueSpec(node *ast.ValueSpec) {
	if node.Type == nil {
		return
	}

	ident, ok := node.Type.(*ast.Ident)
	if !ok {
		return
	}

	typeName := ident.Name
	if _, exists := pc.immutableTypes[typeName]; exists {
		// Associate all variables in this spec with this type name
		for _, name := range node.Names {
			if obj := pc.pass.TypesInfo.ObjectOf(name); obj != nil {
				pc.varToTypeAlias[obj] = typeName
			}
		}
	}
}

// processAssignStmt handles short variable declarations like: x := AliasString("hello")
func (pc *passCollector) processAssignStmt(node *ast.AssignStmt) {
	if node.Tok != token.DEFINE {
		return
	}

	for i, rhs := range node.Rhs {
		if i >= len(node.Lhs) {
			break
		}

		var typeName string

		// Check if RHS is a type conversion
		if call, ok := rhs.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				typeName = ident.Name
			}
		}

		// Check for composite literals like: x := AliasType{...}
		if compLit, ok := rhs.(*ast.CompositeLit); ok {
			if ident, ok := compLit.Type.(*ast.Ident); ok {
				typeName = ident.Name
			}
		}

		if typeName != "" {
			if _, exists := pc.immutableTypes[typeName]; exists {
				if lhsIdent, ok := node.Lhs[i].(*ast.Ident); ok {
					if obj := pc.pass.TypesInfo.ObjectOf(lhsIdent); obj != nil {
						pc.varToTypeAlias[obj] = typeName
					}
				}
			}
		}
	}
}

// trackCopiesAndAliases identifies variables that are copies from map/slice access
// and tracks pointer aliases to immutable fields
func (pc *passCollector) trackCopiesAndAliases() {
	putLog(info, "started tracking copies and aliases pass")

	for _, file := range pc.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok {
				pc.processAssignmentForCopiesAndAliases(assign)
			}
			return true
		})
	}

	putLog(info, "finished tracking copies and aliases pass")
}

// processAssignmentForCopiesAndAliases handles the logic for tracking copies and aliases
func (pc *passCollector) processAssignmentForCopiesAndAliases(assign *ast.AssignStmt) {
	if assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN {
		return
	}

	for i, rhs := range assign.Rhs {
		// Track copies from index expressions
		if _, ok := rhs.(*ast.IndexExpr); ok {
			pc.trackCopyFromIndex(assign, i)
		}

		// Track aliases to immutable fields
		if pc.isAddrOfImmutableField(rhs) {
			pc.markAlias(assign.Lhs, i)
			continue
		}

		// Track aliases with type conversion: p := (*int)(&im.Num)
		if call, ok := rhs.(*ast.CallExpr); ok {
			if len(call.Args) == 1 && pc.isAddrOfImmutableField(call.Args[0]) {
				pc.markAlias(assign.Lhs, i)
			}
		}
	}
}

// trackCopyFromIndex marks variables assigned from index expressions as copies
func (pc *passCollector) trackCopyFromIndex(assign *ast.AssignStmt, i int) {
	if i >= len(assign.Lhs) {
		return
	}

	ident, ok := assign.Lhs[i].(*ast.Ident)
	if !ok {
		return
	}

	obj := pc.pass.TypesInfo.ObjectOf(ident)
	if obj == nil || !isImmutableType(obj.Type(), pc.immutableTypes) {
		return
	}

	// check if not a pointer -> then it's a copy
	if _, isPtr := obj.Type().(*types.Pointer); !isPtr {
		pc.copiedVariables[obj] = true
	}
}

// markAlias marks a variable at the given index as an alias to an immutable field
func (pc *passCollector) markAlias(lhs []ast.Expr, lhsIdx int) {
	if lhsIdx >= len(lhs) {
		return
	}

	ident, ok := lhs[lhsIdx].(*ast.Ident)
	if !ok {
		return
	}

	if obj := pc.pass.TypesInfo.ObjectOf(ident); obj != nil {
		pc.aliasToImmutableField[obj] = true
	}
}

// isAddrOfImmutableField checks if expr is &selector-to-immutable, stripping parens
func (pc *passCollector) isAddrOfImmutableField(expr ast.Expr) bool {
	// strip parentheses
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return pc.isAddrOfImmutableField(paren.X)
	}

	// check for &selector
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return false
	}

	sel, ok := unary.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return getImmutableTypeName(pc.pass, sel, pc.immutableTypes) != ""
}

// checkMutations performs the final pass to detect and report immutable violations
func (pc *passCollector) checkMutations() {
	putLog(info, "started mutation checking pass")

	ctx := &analysisCtx{
		pass:                  pc.pass,
		immutableTypes:        pc.immutableTypes,
		copiedVariables:       pc.copiedVariables,
		aliasToImmutableField: pc.aliasToImmutableField,
		varToTypeAlias:        pc.varToTypeAlias,
		commentGroups:         nil,
	}

	for _, file := range pc.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				ctx.commentGroups = file.Comments
				checkAssignmentWithCopiesAndAliases(ctx, node)
			case *ast.IncDecStmt:
				ctx.commentGroups = file.Comments
				checkIncDecWithCopiesAndAliases(ctx, node)
			}
			return true
		})
	}

	putLog(info, "finished mutation checking pass")
}

func run(pass *analysis.Pass) (any, error) {
	putLog(info, "=====================================")

	if ok, _ := isParserOk(pass); !ok.(bool) {
		putLog(info, "immutablecheck: analysis skipped due to errors in package")
		return nil, nil
	}

	// Create pass collector and run all analysis phases
	collector := newPassCollector(pass)
	collector.firstPass()
	collector.secondPass()
	collector.thirdPass()
	collector.fourthPass()

	return nil, nil
}

func hasImmutableComment(genDecl *ast.GenDecl, _ []*ast.CommentGroup) bool {
	if genDecl.Doc != nil {
		for _, comment := range genDecl.Doc.List {
			text := strings.TrimSpace(comment.Text)
			if strings.Contains(text, "@immutable") {
				return true
			}
		}
	}
	return false
}

// hasAllowMutateComment checks if a statement has an @allow-mutate directive
// The directive MUST be an inline comment directly after the statement on the same line.
// Format: x = "value" //@allow-mutate  OR  x = "value" // @allow-mutate
// Comments on lines above or below the statement are NOT supported.
// ^^^ this just causes a lot of problems with how go AST groups together comments in a CommentGroup
func hasAllowMutateComment(pass *analysis.Pass, pos token.Pos, commentGroups []*ast.CommentGroup) bool {
	stmtPosition := pass.Fset.Position(pos)

	for _, cg := range commentGroups {
		for _, comment := range cg.List {
			commentPos := pass.Fset.Position(comment.Pos())
			text := strings.TrimSpace(comment.Text)

			// Check if this specific comment contains @allow-mutate
			// ONLY allow inline comments on the exact same line as the statement
			if strings.Contains(text, "@allow-mutate") && commentPos.Line == stmtPosition.Line {
				return true
			}
		}
	}
	return false
}

func getImmutableTypeName(pass *analysis.Pass, expr ast.Expr, immutableTypes map[string]immutableInfo) string {
	typ := pass.TypesInfo.TypeOf(expr)
	if typ == nil {
		return ""
	}

	// for index expressions (e.g., arr[0], map["key"]), check the container
	if idx, ok := expr.(*ast.IndexExpr); ok {
		// check the container (e.g., for s.Map["key"], check s)
		if sel, ok := idx.X.(*ast.SelectorExpr); ok {
			// for selector inside index (e.g., s.Map["key"]), check the base object
			if baseIdent, ok := sel.X.(*ast.Ident); ok {
				baseObj := pass.TypesInfo.ObjectOf(baseIdent)
				if baseObj != nil {
					baseType := baseObj.Type()
					if immutableName := getTypeNameFromTypeRecursive(baseType, immutableTypes); immutableName != "" {
						return immutableName
					}
				}
			}
			// also check the selector's type
			if selType := pass.TypesInfo.TypeOf(sel.X); selType != nil {
				if immutableName := getTypeNameFromTypeRecursive(selType, immutableTypes); immutableName != "" {
					return immutableName
				}
			}
		}

		// check the element type
		if elemType := pass.TypesInfo.TypeOf(idx); elemType != nil {
			if immutableName := getTypeNameFromTypeRecursive(elemType, immutableTypes); immutableName != "" {
				return immutableName
			}
		}

		// also check the container type
		if containerType := pass.TypesInfo.TypeOf(idx.X); containerType != nil {
			// for slices/arrays/maps, we need to get the element type
			switch t := containerType.(type) {
			case *types.Slice:
				if immutableName := getTypeNameFromTypeRecursive(t.Elem(), immutableTypes); immutableName != "" {
					return immutableName
				}
			case *types.Array:
				if immutableName := getTypeNameFromTypeRecursive(t.Elem(), immutableTypes); immutableName != "" {
					return immutableName
				}
			case *types.Map:
				if immutableName := getTypeNameFromTypeRecursive(t.Elem(), immutableTypes); immutableName != "" {
					return immutableName
				}
			}
		}

		// recursively check the container for nested immutables
		return getImmutableTypeName(pass, idx.X, immutableTypes)
	}

	// for selector expressions (e.g., im.Field), check the base object
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		// first check the parent (X) type
		parentType := pass.TypesInfo.TypeOf(sel.X)
		if parentType != nil {
			if immutableName := getTypeNameFromTypeRecursive(parentType, immutableTypes); immutableName != "" {
				return immutableName
			}
		}
	}

	// remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// vheck if it's a named type
	if named, ok := typ.(*types.Named); ok {
		typeName := named.Obj().Name()
		if _, exists := immutableTypes[typeName]; exists {
			return typeName
		}

		// check for embedded immutable fields
		if structType, ok := named.Underlying().(*types.Struct); ok {
			for i := 0; i < structType.NumFields(); i++ {
				field := structType.Field(i)
				if field.Embedded() {
					embeddedTypeName := getTypeNameFromType(field.Type())
					if _, exists := immutableTypes[embeddedTypeName]; exists {
						return embeddedTypeName
					}
				}
			}
		}
	}

	return getTypeNameFromTypeRecursive(typ, immutableTypes)
}

func getTypeNameFromType(typ types.Type) string {
	// remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		return named.Obj().Name()
	}

	return ""
}

func getTypeNameFromTypeRecursive(typ types.Type, immutableTypes map[string]immutableInfo) string {
	// remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		typeName := named.Obj().Name()
		if _, exists := immutableTypes[typeName]; exists {
			return typeName
		}
	}

	return ""
}

type analysisCtx struct {
	pass                  *analysis.Pass
	immutableTypes        map[string]immutableInfo
	copiedVariables       map[types.Object]bool
	aliasToImmutableField map[types.Object]bool
	varToTypeAlias        map[types.Object]string
	commentGroups         []*ast.CommentGroup
}

func checkAssignmentWithCopiesAndAliases(ctx *analysisCtx, stmt *ast.AssignStmt) {
	// Check if this statement has an @allow-mutate directive
	if hasAllowMutateComment(ctx.pass, stmt.Pos(), ctx.commentGroups) {
		return // Skip this mutation check
	}

	// skip variable declarations (:= token)
	// we only care about mutations, not initial assignments
	// also like if initial assignments were not allowed then like how do I even code?
	if stmt.Tok == token.DEFINE {
		return // := is declaration, not mutation
	}

	for i, lhs := range stmt.Lhs {
		// check if LHS is just an identifier (simple variable assignment like: x = value)
		if ident, ok := lhs.(*ast.Ident); ok {
			// This is a simple variable assignment, not a field mutation
			// We need to distinguish:
			// 1. im = Immtbl{} (reassigning whole immutable value - should CATCH)
			// 2. imPtr = &im (assigning pointer - should NOT catch, it's read-only)
			// 3. val = mapOfImmutables["key"] (assigning copy - should also NOT catch)
			// 4. imPtr, i = multi() where imPtr is *Imm (multi-value with pointer - should NOT catch)
			// 5. t, y = m() where t is im value type (multi-value with value - should CATCH)
			if isImmutableVariable(ctx.pass, ident, ctx.immutableTypes, ctx.varToTypeAlias) {
				// Check if the variable is a pointer type
				// If it's a pointer to an immutable type, reassigning the pointer is OK
				// Only catch reassignment of immutable VALUES
				obj := ctx.pass.TypesInfo.ObjectOf(ident)
				if obj != nil {
					if _, isPtr := obj.Type().(*types.Pointer); isPtr {
						// This is reassigning a pointer variable - allow it
						// This handles both single and multi-value assignments
						continue
					}
				}

				// check RHS - if it's just taking address, don't flag (read-only operation)
				if i < len(stmt.Rhs) {
					rhs := stmt.Rhs[i]
					// allow pointer assignments like: result = &im
					if unary, ok := rhs.(*ast.UnaryExpr); ok && unary.Op == token.AND {
						continue
					}
				}

				// this is reassigning the whole immutable struct - flag it
				reportMutation(ctx.pass, stmt.Pos(), ident.Name, lhs, ctx.immutableTypes, "reassigning whole immutable struct")
			}
			continue
		}

		// check if LHS is a selector (field access like im.Num or val.Num)
		if sel, ok := lhs.(*ast.SelectorExpr); ok {
			// check if the base is a copied variable
			if base, ok := sel.X.(*ast.Ident); ok {
				baseObj := ctx.pass.TypesInfo.ObjectOf(base)
				if baseObj != nil && ctx.copiedVariables[baseObj] {
					// this is mutating a copy - skip it
					continue
				}
			}
		}

		// for all other LHS patterns, check if it's an immutable mutation
		if isImmutableMutationWithAliases(ctx.pass, lhs, ctx.immutableTypes, ctx.aliasToImmutableField, ctx.varToTypeAlias) {
			reportMutation(ctx.pass, stmt.Pos(), getExpressionString(lhs), lhs, ctx.immutableTypes, "mutating immutable field in assignment")
		}
	}
}

func checkIncDecWithCopiesAndAliases(ctx *analysisCtx, stmt *ast.IncDecStmt) {
	// Check if this statement has an @allow-mutate directive
	if hasAllowMutateComment(ctx.pass, stmt.Pos(), ctx.commentGroups) {
		return // Skip this mutation check
	}

	// check if we're incrementing/decrementing a field of a copied variable
	if sel, ok := stmt.X.(*ast.SelectorExpr); ok {
		if base, ok := sel.X.(*ast.Ident); ok {
			baseObj := ctx.pass.TypesInfo.ObjectOf(base)
			if baseObj != nil && ctx.copiedVariables[baseObj] {
				// this is mutating a copy - skip it
				return
			}
		}
	}

	if isImmutableMutationWithAliases(ctx.pass, stmt.X, ctx.immutableTypes, ctx.aliasToImmutableField, ctx.varToTypeAlias) {
		reportMutation(ctx.pass, stmt.Pos(), getExpressionString(stmt.X), stmt.X, ctx.immutableTypes, "incrementing/decrementing immutable field")
	}
}

// stripParens recursively unwraps parenthesized expressions
func stripParens(expr ast.Expr) ast.Expr {
	for {
		if paren, ok := expr.(*ast.ParenExpr); ok {
			expr = paren.X
		} else {
			return expr
		}
	}
}

func isImmutableMutationWithAliases(pass *analysis.Pass, expr ast.Expr, immutableTypes map[string]immutableInfo, aliasToImmutableField map[types.Object]bool, varToTypeAlias map[types.Object]string) bool {
	// Strip all parentheses before checking
	expr = stripParens(expr)

	switch e := expr.(type) {
	case *ast.SelectorExpr:
		// Check if we're accessing a field of an immutable struct
		// Need to handle both direct access (im.Field) and nested access (outer.Inner.Field)

		// Strip parens from the base expression
		x := stripParens(e.X)

		if ident, ok := x.(*ast.Ident); ok {
			// Direct field access: check if the base variable is immutable
			if isImmutableVariable(pass, ident, immutableTypes, varToTypeAlias) {
				return true
			}
			// Also check if this is accessing a field from an embedded immutable type
			baseType := pass.TypesInfo.TypeOf(ident)
			if baseType != nil {
				if structType, ok := getStructType(baseType); ok {
					// Check if the field being accessed comes from an embedded immutable
					fieldName := e.Sel.Name
					if isFieldFromEmbeddedImmutable(structType, fieldName, immutableTypes) {
						return true
					}
				}
			}
			return false
		} else if _, ok := x.(*ast.SelectorExpr); ok {
			// Nested field access: check if any intermediate type is immutable
			// First check if the parent selector itself refers to an immutable type
			parentType := pass.TypesInfo.TypeOf(x)
			if parentType != nil && isImmutableType(parentType, immutableTypes) {
				return true
			}
			// Then recursively check the parent selector
			return isImmutableMutationWithAliases(pass, x, immutableTypes, aliasToImmutableField, varToTypeAlias)
		} else if _, ok := x.(*ast.IndexExpr); ok {
			// Handle mutations like: mapOfImmutablePtrs["key"].Num
			// or arr[0].Num
			parentType := pass.TypesInfo.TypeOf(x)
			if parentType != nil && isImmutableType(parentType, immutableTypes) {
				return true
			}
			return isImmutableMutationWithAliases(pass, x, immutableTypes, aliasToImmutableField, varToTypeAlias)
		} else if _, ok := x.(*ast.CallExpr); ok {
			// Handle mutations like: getImmutable().Num
			returnType := pass.TypesInfo.TypeOf(x)
			if returnType != nil && isImmutableType(returnType, immutableTypes) {
				return true
			}
		} else if _, ok := x.(*ast.StarExpr); ok {
			// Handle mutations like: (*ptr).Num or dereferenced pointers
			derefType := pass.TypesInfo.TypeOf(x)
			if derefType != nil && isImmutableType(derefType, immutableTypes) {
				return true
			}
		} else if typeAssert, ok := x.(*ast.TypeAssertExpr); ok {
			// Handle type assertions: iface.(*Immtbl).Num
			assertedType := pass.TypesInfo.TypeOf(typeAssert)
			if assertedType != nil && isImmutableType(assertedType, immutableTypes) {
				return true
			}
		}

		// Fallback: check the type of the base itself
		xType := pass.TypesInfo.TypeOf(x)
		if xType != nil && isImmutableType(xType, immutableTypes) {
			return true
		}

	case *ast.IndexExpr:
		// Check if we're indexing into an immutable array/slice/map
		// Handle: arr[0].Num or mapOfImmutablePtrs["key"].Num
		indexedType := pass.TypesInfo.TypeOf(e)
		if indexedType != nil && isImmutableType(indexedType, immutableTypes) {
			return true
		}
		// Also check the container itself (strip parens first)
		return isImmutableMutationWithAliases(pass, stripParens(e.X), immutableTypes, aliasToImmutableField, varToTypeAlias)

	case *ast.StarExpr:
		// Handle pointer dereferences: *ptr = value or (*ptr).Field
		// Strip parens from the pointer expression
		x := stripParens(e.X)

		// Check the dereferenced type
		derefType := pass.TypesInfo.TypeOf(e)
		if derefType != nil && isImmutableType(derefType, immutableTypes) {
			return true
		}
		// NEW: check if *ident where ident is known to alias an immutable field
		if ident, ok := x.(*ast.Ident); ok {
			if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
				if aliasToImmutableField[obj] {
					return true
				}
			}
		}
		// Also recursively check the pointer expression
		return isImmutableMutationWithAliases(pass, x, immutableTypes, aliasToImmutableField, varToTypeAlias)

	case *ast.ParenExpr:
		// Should not reach here due to stripParens at entry, but handle anyway
		return isImmutableMutationWithAliases(pass, e.X, immutableTypes, aliasToImmutableField, varToTypeAlias)

	case *ast.UnaryExpr:
		// Handle unary expressions like &im.Num or *ptr
		switch e.Op {
		case token.AND:
			// Taking address: check if we're taking address of immutable field
			return isImmutableMutationWithAliases(pass, stripParens(e.X), immutableTypes, aliasToImmutableField, varToTypeAlias)
		case token.MUL:
			// Dereferencing: check the dereferenced type
			derefType := pass.TypesInfo.TypeOf(e)
			if derefType != nil && isImmutableType(derefType, immutableTypes) {
				return true
			}
		default:
			// do nothing
		}

	case *ast.Ident:
		// direct mutation of immutable variable
		return isImmutableVariable(pass, e, immutableTypes, varToTypeAlias)
	}
	return false
}

func getStructType(typ types.Type) (*types.Struct, bool) {
	// remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// get the underlying struct
	if named, ok := typ.(*types.Named); ok {
		if structType, ok := named.Underlying().(*types.Struct); ok {
			return structType, true
		}
	}

	if structType, ok := typ.(*types.Struct); ok {
		return structType, true
	}

	return nil, false
}

func isFieldFromEmbeddedImmutable(structType *types.Struct, fieldName string, immutableTypes map[string]immutableInfo) bool {
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if field.Embedded() {
			// check if this embedded field is immutable
			if isImmutableType(field.Type(), immutableTypes) {
				// check if the embedded type has this field
				if embeddedStruct, ok := getStructType(field.Type()); ok {
					for j := 0; j < embeddedStruct.NumFields(); j++ {
						if embeddedStruct.Field(j).Name() == fieldName {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func isImmutableVariable(pass *analysis.Pass, ident *ast.Ident, immutableTypes map[string]immutableInfo, varToTypeAlias map[types.Object]string) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	// First check if this variable was declared with an immutable type alias
	// This handles type aliases like: type AliasString = string
	if typeName, exists := varToTypeAlias[obj]; exists {
		if _, isImmutable := immutableTypes[typeName]; isImmutable {
			return true
		}
	}

	// Then check if the variable's type is a named immutable type
	// This handles type definitions like: type ImmutableString string
	if varObj, ok := obj.(*types.Var); ok {
		// Get the type name as it appears in the source
		if named, ok := varObj.Type().(*types.Named); ok {
			typeName := named.Obj().Name()
			if _, exists := immutableTypes[typeName]; exists {
				return true
			}
		}
	}

	typ := obj.Type()
	return isImmutableType(typ, immutableTypes)
}

func isImmutableType(typ types.Type, immutableTypes map[string]immutableInfo) bool {
	// remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// Handle type aliases (Go 1.22+)
	// For type aliases like: type Alias = Immtbl
	// The type will be *types.Alias, and we need to resolve it to the actual type
	// Type aliases in go are "transparent", so we need to get the Rhs type
	if alias, ok := typ.(interface{ Rhs() types.Type }); ok {
		// get the right-hand side of the alias (the actual type)
		actualType := alias.Rhs()
		return isImmutableType(actualType, immutableTypes)
	}

	// check if it's a named type
	if named, ok := typ.(*types.Named); ok {
		typeName := named.Obj().Name()

		// First, check if this exact type name is marked as immutable
		_, exists := immutableTypes[typeName]
		if exists {
			return true
		}

		// for type aliases, the underlying type will be the aliased type
		// we need to check if the underlying type is also a named type that's immutable
		underlying := named.Underlying()

		// check if the underlying type is a struct with embedded immutable fields
		if structType, ok := underlying.(*types.Struct); ok {
			for i := 0; i < structType.NumFields(); i++ {
				field := structType.Field(i)
				if field.Embedded() {
					// Check if embedded field is immutable
					if isImmutableType(field.Type(), immutableTypes) {
						return true
					}
				}
			}
		}

		// Additionally, check all types in immutableTypes to see if any match this underlying structure
		// This handles type aliases: type Alias = Immtbl
		for immutableTypeName := range immutableTypes {
			// for each immutable type, check if it has the same underlying structure
			// we do this by checking if the package and type structure match
			if named.Obj().Pkg() != nil {
				// try to find the immutable type in the same package scope
				pkgScope := named.Obj().Pkg().Scope()
				if immutableObj := pkgScope.Lookup(immutableTypeName); immutableObj != nil {
					if immutableTypeObj, ok := immutableObj.(*types.TypeName); ok {
						immutableType := immutableTypeObj.Type()

						// Handle both *types.Named and *types.Alias
						var immutableUnderlying types.Type
						if immutableNamed, ok := immutableType.(*types.Named); ok {
							immutableUnderlying = immutableNamed.Underlying()
						} else if alias, ok := immutableType.(interface{ Rhs() types.Type }); ok {
							// For type aliases, get the RHS (underlying type)
							immutableUnderlying = alias.Rhs()
						} else {
							continue
						}

						// check if the underlying types are identical
						if types.Identical(underlying, immutableUnderlying) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}
