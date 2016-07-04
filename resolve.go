package wire9

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

type named struct {
	string
}

type cleaner struct{}

type counter int

func NewCleaner() *cleaner {
	return new(cleaner)
}

// Visit traverses a node looking for redundant block statements
// within function declarations. It removes redundant bracing injected
// by the generator.
func (c *cleaner) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	switch t := n.(type) {
	case *ast.FuncDecl:
		body := &ast.BlockStmt{List: make([]ast.Stmt, 0)}
		for _, v := range t.Body.List {
			switch v := v.(type) {
			case *ast.ForStmt:
			case *ast.BlockStmt:
				var count counter
				ast.Walk(&count, v)
				if count > 0 {
					body.List = append(body.List, v)
				} else {
					body.List = append(body.List, v.List...)
				}
			case ast.Stmt, ast.Expr, ast.Node:
				body.List = append(body.List, v)
			}
		}
		t.Body = body
	}
	return c
}

func (c *counter) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	switch t := n.(type) {
	case *ast.AssignStmt:
		if t.Tok == token.DEFINE {
			*c++
		}
	case *ast.SwitchStmt, *ast.TypeSwitchStmt,
		*ast.SelectStmt, *ast.IfStmt, *ast.ForStmt:
		return nil
	}
	return c
}

// Endian returns the endianness of a field. TODO: return
// an error if the field can't have an endian value.
func Endian(m ast.Node) (string, error) {
	switch t := m.(type) {
	case *ast.Ident:
		switch t.Name{
		case "BE": return "binary.BigEndian", nil
		case "LE": return "binary.LittleEndian", nil
		default:   return "", fmt.Errorf("invalid endian: %s", t.Name)
		}
	case nil:
		return "binary.LittleEndian", nil
	}
	return "binary.LittleEndian", nil
}

func (n *named) Visit(node ast.Node) ast.Visitor {
	switch t := node.(type) {
	case *ast.Ident:
		n.string += string("int(z." + t.Name + ")")
		return nil
	case *ast.BinaryExpr:
		n.Visit(t.X)
		n.string += fmt.Sprintf("%s", t.Op)
		return n.Visit(t.Y)
	case *ast.BasicLit:
		n.string += string(t.Value)
		return nil
	}
	return nil
}

//
// Formatters

// Access returns the named field's accessor expression as a string
func Access(a ast.Expr) string {
	switch a.(type) {
	case *ast.BasicLit:
		return types.ExprString(a)
	case *ast.Ident:
		return fmt.Sprintf("z.%s", types.ExprString(a))
	case *ast.BinaryExpr, *ast.ArrayType, *ast.StarExpr:
		var n named
		ast.Walk(&n, a)
		return fmt.Sprintf("%s", n.string)
	}
	return ""
}

// TypeString returns the named field's type as a string
func TypeString(f interface{}) string {
	switch t := f.(type) {
	case *ast.ArrayType:
		return fmt.Sprintf("[]%s", TypeString(t.Elt))
	case *ast.Ident:
		return fmt.Sprint(t.Name)
	case ast.Expr:
		return types.ExprString(t)
	case *ast.Field:
		return TypeString(t.Type)
	}
	panic("BUG")
}

// WidthOf returns the named field's width as a string
func WidthOf(ts *ast.TypeSpec, f *ast.Field) (s string, err error) {
	if f == nil || f.Names == nil {
		return "", fmt.Errorf("field is nil: %#v", f)
	}
	info := TInfo.Get(ts, f)
	if info == nil {
		return "", fmt.Errorf("field width is nil: %#v", f)
	}
	s = types.ExprString(info.Width)

	switch t := info.Width.(type) {
	case *ast.BasicLit:
		return s, nil
	case *ast.Ident:
		return fmt.Sprintf("int(z.%s)", s), nil
	case *ast.BinaryExpr, *ast.ArrayType, *ast.StarExpr:
		s = Access(t)
	}
	if s == "" {
		return "", fmt.Errorf("field has unspecified width")
	}
	return s, nil
}

//
// Loop detection

type nesting int

// Nesting tracks the level of expression nesting during code generation
var Nesting nesting

func (n *nesting) Inc(b bool) bool {
	if !b {
		return false
	}
	*n++
	return true
}
func (n *nesting) Dec(b bool) bool {
	if !b {
		return false
	}
	*n--
	return true
}

//
// Identity Functions

// Array returns true if f is an array type
func Array(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return f.(*ast.ArrayType).Len != nil
}

// ByteArray returns true if f is an array of bytes
func ByteArray(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return Array(f) && f.(*ast.ArrayType).Elt.(*ast.Ident).Name == "byte"
}

// ByteSlice returns true if f a slice of bytes
func ByteSlice(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return Slice(f) && f.(*ast.ArrayType).Elt.(*ast.Ident).Name == "byte"
}

// Coherent returns true if f is not an aggregate (slice or array) type
func Coherent(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return !Slice(f) && !Array(f)
}

// Custom returns true if f is not a builtin type
func Custom(f ast.Expr) bool {
	return !builtin[TypeString(f)]
}

// CustomSlice returns true if f is not a builtin type, and is a slice
func CustomSlice(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return Slice(f) && Custom(f.(*ast.ArrayType).Elt)
}

// Literal returns true if f is not a builtin type, and is a slice
func Literal(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	_ = f.(*ast.BasicLit)
	return true
}

// Numeric returns true if f is a builtin numeric type
func Numeric(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return builtin[TypeString(f)] && !String(f) && Coherent(f)
}

// Slice returns true if f is a slice type
func Slice(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return f.(*ast.ArrayType).Len == nil
}

// Selector returns the syntax for selecting an element of f. If f
// is not an array or slice, the field is returned as is.
func Selector(f ast.Expr) string {
	s := TypeString(f)
	if Slice(f) || Array(f) {
		return fmt.Sprintf("%s[i]", s)
	}
	return s
}

// SliceOf returns the syntax of f as a slice element
func SliceOf(f ast.Expr) ast.Expr {
	return &ast.ArrayType{
		Len: nil,
		Elt: f,
	}
}

// String returns f's string representation
func String(f ast.Expr) (ok bool) {
	defer func() { recover() }()
	return f.(*ast.Ident).Name == "string"
}
