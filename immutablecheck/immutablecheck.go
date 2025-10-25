package immutablecheck

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var microDeltaIter = NewMicroDeltaIterator()

var Analyzer = &analysis.Analyzer{
	Name:     "plsdontgo",
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

func run(pass *analysis.Pass) (any, error) {
	putLog(info, "=====================================")

	immutableTypes := make(map[string]immutableInfo)

	// need to track copied variables from map/slice access
	// otherwise it can throw a false positive on a copy
	copiedVariables := make(map[types.Object]bool)

	putLog(info, "started first pass")

	// collect @immutable marked types
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				// check for type declaration with `@immutable` comment
				if node.Tok == token.TYPE && hasImmutableComment(node, file.Comments) {
					for _, spec := range node.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							typeName := typeSpec.Name.Name
							immutableTypes[typeName] = immutableInfo{
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
	putLog(info, "finished first pass")
	putLog(dbug, Pretty_print_immutables(&immutableTypes))
	putLog(info, "started tracking copies pass")

	// Track which variables are copies from map/slice access
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok {
				if assign.Tok == token.DEFINE {
					// Check if RHS is an index expression (map/slice access)
					for i, rhs := range assign.Rhs {
						if _, ok := rhs.(*ast.IndexExpr); ok {
							// LHS is being assigned from an index - it is mayhaps a copy
							if i < len(assign.Lhs) {
								if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
									obj := pass.TypesInfo.ObjectOf(ident)
									if obj != nil && isImmutableType(obj.Type(), immutableTypes) {
										// check if not a pointer -> then it's a copy
										if _, isPtr := obj.Type().(*types.Pointer); !isPtr {
											copiedVariables[obj] = true
										}
									}
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	putLog(info, "finished tracking copies pass")

	putLog(info, "started second pass")
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				checkAssignmentWithCopies(pass, node, immutableTypes, copiedVariables)
			case *ast.IncDecStmt:
				checkIncDecWithCopies(pass, node, immutableTypes, copiedVariables)
			}
			return true
		})
	}
	putLog(info, "finished second pass")

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

func checkAssignmentWithCopies(pass *analysis.Pass, stmt *ast.AssignStmt, immutableTypes map[string]immutableInfo, copiedVariables map[types.Object]bool) {
	// Skip variable declarations (:= token)
	// We only care about mutations, not initial assignments
	// also like if initial assignments were not allowed then like how do I even code?
	if stmt.Tok == token.DEFINE {
		return // := is declaration, not mutation
	}

	for i, lhs := range stmt.Lhs {
		// check if LHS is just an identifier (simple variable assignment like: x = value)
		if ident, ok := lhs.(*ast.Ident); ok {
			// This is a simple variable assignment, not a field mutation
			// We need to distinguish:
			// 1. im = Immtbl{} (reassigning whole immutable - should CATCH)
			// 2. result = &im (assigning pointer - should NOT catch, it's read-only yet)
			// 3. val = mapOfImmutables["key"] (assigning copy - should also NOT catch)

			if isImmutableVariable(pass, ident, immutableTypes) {
				// check RHS - if it's just taking address, don't flag (read-only operation)
				if i < len(stmt.Rhs) {
					rhs := stmt.Rhs[i]
					// allow pointer assignments like: result = &im
					if unary, ok := rhs.(*ast.UnaryExpr); ok && unary.Op == token.AND {
						continue
					}
				}
				// this is reassigning the whole immutable struct - flag it
				reportMutation(pass, stmt.Pos(), ident.Name, lhs, immutableTypes, "reassigning whole immutable struct")
			}
			continue
		}

		// Check if LHS is a selector (field access like im.Num or val.Num)
		if sel, ok := lhs.(*ast.SelectorExpr); ok {
			// Check if the base is a copied variable
			if base, ok := sel.X.(*ast.Ident); ok {
				baseObj := pass.TypesInfo.ObjectOf(base)
				if baseObj != nil && copiedVariables[baseObj] {
					// This is mutating a copy - skip it
					continue
				}
			}
		}

		// For all other LHS patterns, check if it's an immutable mutation
		if isImmutableMutation(pass, lhs, immutableTypes) {
			reportMutation(pass, stmt.Pos(), getExpressionString(lhs), lhs, immutableTypes, "")
		}
	}
}

func checkIncDecWithCopies(pass *analysis.Pass, stmt *ast.IncDecStmt, immutableTypes map[string]immutableInfo, copiedVariables map[types.Object]bool) {
	// Check if we're incrementing/decrementing a field of a copied variable
	if sel, ok := stmt.X.(*ast.SelectorExpr); ok {
		if base, ok := sel.X.(*ast.Ident); ok {
			baseObj := pass.TypesInfo.ObjectOf(base)
			if baseObj != nil && copiedVariables[baseObj] {
				// This is mutating a copy - skip it
				return
			}
		}
	}

	if isImmutableMutation(pass, stmt.X, immutableTypes) {
		reportMutation(pass, stmt.Pos(), getExpressionString(stmt.X), stmt.X, immutableTypes, "incrementing/decrementing immutable field")
	}
}

func isImmutableMutation(pass *analysis.Pass, expr ast.Expr, immutableTypes map[string]immutableInfo) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		// Check if we're accessing a field of an immutable struct
		// Need to handle both direct access (im.Field) and nested access (outer.Inner.Field)
		if x, ok := e.X.(*ast.Ident); ok {
			// Direct field access: check if the base variable is immutable
			if isImmutableVariable(pass, x, immutableTypes) {
				return true
			}
			// Also check if this is accessing a field from an embedded immutable type
			baseType := pass.TypesInfo.TypeOf(x)
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
		} else if _, ok := e.X.(*ast.SelectorExpr); ok {
			// Nested field access: check if any intermediate type is immutable
			// First check if the parent selector itself refers to an immutable type
			parentType := pass.TypesInfo.TypeOf(e.X)
			if parentType != nil && isImmutableType(parentType, immutableTypes) {
				return true
			}
			// Then recursively check the parent selector
			return isImmutableMutation(pass, e.X, immutableTypes)
		} else if _, ok := e.X.(*ast.IndexExpr); ok {
			// Handle mutations like: mapOfImmutablePtrs["key"].Num
			// or arr[0].Num
			parentType := pass.TypesInfo.TypeOf(e.X)
			if parentType != nil && isImmutableType(parentType, immutableTypes) {
				return true
			}
			return isImmutableMutation(pass, e.X, immutableTypes)
		} else if _, ok := e.X.(*ast.CallExpr); ok {
			// Handle mutations like: getImmutable().Num
			returnType := pass.TypesInfo.TypeOf(e.X)
			if returnType != nil && isImmutableType(returnType, immutableTypes) {
				return true
			}
		} else if _, ok := e.X.(*ast.StarExpr); ok {
			// Handle mutations like: (*ptr).Num or dereferenced pointers
			derefType := pass.TypesInfo.TypeOf(e.X)
			if derefType != nil && isImmutableType(derefType, immutableTypes) {
				return true
			}
		} else if _, ok := e.X.(*ast.ParenExpr); ok {
			// Handle parenthesized expressions
			return isImmutableMutation(pass, e.X, immutableTypes)
		} else if typeAssert, ok := e.X.(*ast.TypeAssertExpr); ok {
			// Handle type assertions: iface.(*Immtbl).Num
			assertedType := pass.TypesInfo.TypeOf(typeAssert)
			if assertedType != nil && isImmutableType(assertedType, immutableTypes) {
				return true
			}
		}

		// Fallback: check the type of e.X itself
		xType := pass.TypesInfo.TypeOf(e.X)
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
		// Also check the container itself
		return isImmutableMutation(pass, e.X, immutableTypes)

	case *ast.StarExpr:
		// Handle pointer dereferences: *ptr = value or (*ptr).Field
		// Check the dereferenced type
		derefType := pass.TypesInfo.TypeOf(e)
		if derefType != nil && isImmutableType(derefType, immutableTypes) {
			return true
		}
		// Also recursively check the pointer expression
		return isImmutableMutation(pass, e.X, immutableTypes)

	case *ast.ParenExpr:
		// Handle parenthesized expressions: (*&im.Num) or (im).Num
		return isImmutableMutation(pass, e.X, immutableTypes)

	case *ast.UnaryExpr:
		// Handle unary expressions like &im.Num or *ptr
		if e.Op == token.AND {
			// Taking address: check if we're taking address of immutable field
			return isImmutableMutation(pass, e.X, immutableTypes)
		} else if e.Op == token.MUL {
			// Dereferencing: check the dereferenced type
			derefType := pass.TypesInfo.TypeOf(e)
			if derefType != nil && isImmutableType(derefType, immutableTypes) {
				return true
			}
		}

	case *ast.Ident:
		// Direct mutation of immutable variable
		return isImmutableVariable(pass, e, immutableTypes)
	}
	return false
}

// Helper function to get struct type from a type (handling pointers)
func getStructType(typ types.Type) (*types.Struct, bool) {
	// Remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// Get the underlying struct
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

// Helper function to check if a field comes from an embedded immutable type
func isFieldFromEmbeddedImmutable(structType *types.Struct, fieldName string, immutableTypes map[string]immutableInfo) bool {
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if field.Embedded() {
			// Check if this embedded field is immutable
			if isImmutableType(field.Type(), immutableTypes) {
				// Check if the embedded type has this field
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

func isImmutableVariable(pass *analysis.Pass, ident *ast.Ident, immutableTypes map[string]immutableInfo) bool {
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}

	typ := obj.Type()
	return isImmutableType(typ, immutableTypes)
}

func isImmutableType(typ types.Type, immutableTypes map[string]immutableInfo) bool {
	// Remove pointer indirection
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// Handle type aliases (Go 1.22+)
	// For type aliases like: type Alias = Immtbl
	// The type will be *types.Alias, and we need to resolve it to the actual type
	if alias, ok := typ.(interface{ Rhs() types.Type }); ok {
		// Get the right-hand side of the alias (the actual type)
		actualType := alias.Rhs()
		return isImmutableType(actualType, immutableTypes)
	}

	// Check if it's a named type
	if named, ok := typ.(*types.Named); ok {
		typeName := named.Obj().Name()

		// First, check if this exact type name is marked as immutable
		_, exists := immutableTypes[typeName]
		if exists {
			return true
		}

		// For type aliases, the underlying type will be the aliased type
		// We need to check if the underlying type is also a named type that's immutable
		underlying := named.Underlying()

		// Check if the underlying type is a struct with embedded immutable fields
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
			// For each immutable type, check if it has the same underlying structure
			// We do this by checking if the package and type structure match
			if named.Obj().Pkg() != nil {
				// Try to find the immutable type in the same package scope
				pkgScope := named.Obj().Pkg().Scope()
				if immutableObj := pkgScope.Lookup(immutableTypeName); immutableObj != nil {
					if immutableTypeObj, ok := immutableObj.(*types.TypeName); ok {
						immutableNamedType := immutableTypeObj.Type().(*types.Named)
						// Check if the underlying types are identical
						if types.Identical(underlying, immutableNamedType.Underlying()) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}
