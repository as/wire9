package wire9

import (
	"fmt"
	"go/ast"
)

// There are two kinds of duplicates: A wire definition or a go struct/method.
// The former is not allowed and should stop the parser. The latter is more
// complicated:
//
// If a Go struct or method conflicts with a wire definition, the following
// will happen.
//
// 1. The source file is examined by name. If the name contains _wire9.go, then
//    the duplicate is ignored. It will be overwritten anyway.
//
// 2. If the name does not contain _wire9.go, any struct or method
//    contained in the wire definition with a duplicate is not regenerated.

// Dup marks the type of duplicate found in the original source code. A
// that a wire definition would likely conflict with.
type (
	Dup struct{
		name string
		kind string
	}
	DupMap map[Dup]*ast.Ident
)


// Dupfind walks the AST looking for structs and method
// sets that could conflict with a wire definition.
func DupFind(n ast.Node) DupMap {
	v := make(DupMap)
	ast.Walk(&v, n)
	return v
}

// Visit traverses the AST
func (v DupMap) Visit(n ast.Node) ast.Visitor {
	// We're looking for two things:

	switch n := n.(type) {
	case *ast.Package, *ast.File, *ast.GenDecl:
		return v
	case *ast.TypeSpec:
		// A struct type not in a *_wire9.go file that would conflict with
		// a wire definition.
		if _, ok := n.Type.(*ast.StructType); ok {
			v[Dup{n.Name.Name, "StructType"}] = n.Name
			fmt.Println("Found struct definition for", n.Name)
		}
		return v
	case *ast.FuncDecl:
		// A ReadBinary or WriteBinary method of such a struct
		if n.Recv == nil {
			return nil
		}
		recv := fmt.Sprint(n.Recv.List[0].Type)
		fn := n.Name.Name
		if fn == "WriteBinary" {
			v[Dup{fn, "WriteBinary"}] = n.Name
		} else if fn == "ReadBinary" {
			v[Dup{fn, "ReadBinary"}] = n.Name
		}
		fmt.Println("Found", fn, "for", recv)
		return nil
	}
	return nil
}

func (v DupMap) Merge(v2 DupMap) {
	for key, val := range v2{
		v[key] = val
	}
}

