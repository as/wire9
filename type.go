package wire9

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
)

// A Mode value is a set of flags (or 0).
// They control the amount of source code parsed and other optional
// parser functionality.
//
type Mode uint

// SizeType maps sizes (as string values) to
// common datatypes of that size
var sizeType = map[int]ast.Expr{
	0: &ast.StructType{},
	1: &ast.Ident{Name: "byte"},
	2: &ast.Ident{Name: "uint16"},
	4: &ast.Ident{Name: "uint32"},
	8: &ast.Ident{Name: "uint64"},
}

var info = types.Info{
	Types: make(map[ast.Expr]types.TypeAndValue),
	Defs:  make(map[*ast.Ident]types.Object),
	Uses:  make(map[*ast.Ident]types.Object),
}

// TypeFromWidth determines the type by the width
func TypeFromWidth(w ast.Expr) (ast.Expr, error) {
	switch t := w.(type) {
	case *ast.Ident:
		return byteSlice(), nil
	case *ast.BasicLit:
		num, err := strconv.Atoi(t.Value)
		if err != nil {
			return nil, err
		}
		typ, ok := sizeType[num]
		if ok {
			return typ, nil
		}
		return byteSlice(), nil
	case *ast.BinaryExpr:
		return byteSlice(), nil
	}
	return nil, fmt.Errorf("cant get type from width: %s", w)
}

func byteSlice() ast.Expr {
	return SliceOf(&ast.Ident{Name: "byte"})
}

var builtin = map[string]bool{
	"byte":       true,
	"bool":       true,
	"int":        true,
	"rune":       true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
	"float32":    true,
	"float64":    true,
	"complex64":  true,
	"complex128": true,
	"string":     true,
}
