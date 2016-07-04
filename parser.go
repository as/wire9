package wire9

import (
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"strings"
)

// Parser flags
const (
	PackageClauseOnly Mode             = 1 << iota // stop parsing after package clause
	ImportsOnly                                    // stop parsing after import declarations
	ParseComments                                  // parse comments and add them to AST
	Trace                                          // print a trace of parsed productions
	DeclarationErrors                              // report declaration errors
	SpuriousErrors                                 // same as AllErrors, for backward-compatibility
	AllErrors         = SpuriousErrors             // report all errors (not just the first 10 on different lines)
)

// The parser structure holds the parser's internal state.
type parser struct {
	file    *token.File
	errors  scanner.ErrorList
	scanner scanner.Scanner

	// Tracing/debugging
	mode   Mode // parsing mode
	trace  bool // == (mode & Trace != 0)
	indent int  // indentation used for tracing output

	// Next token
	pos token.Pos   // token position
	tok token.Token // one token look-ahead
	lit string      // token literal

	// Ordinary identifier scopes
	pkgScope   *ast.Scope        // pkgScope.Outer == nil
	topScope   *ast.Scope        // top-most scope; may be pkgScope
	unresolved []*ast.Ident      // unresolved identifiers
	imports    []*ast.ImportSpec // list of imports

	// Label scopes
	// (maintained by open/close LabelScope)
	labelScope  *ast.Scope     // label scope for current function
	targetStack [][]*ast.Ident // stack of unresolved labels
	// Non-syntactic parser control
	exprLev int  // < 0: in control clause, >= 0: in expression
	inRHS   bool // if set, the parser is parsing a rhs expression

	DupMap    DupMap
	intDupMap DupMap
}

// NewParser returns an initialized parser.
func NewParser(fset *token.FileSet, line string, dups DupMap) (p *parser, err error) {
	if !strings.HasPrefix(line, "//wire9") {
		return nil, fmt.Errorf("not a comment")
	}
	line = line[7:]
	fset = token.NewFileSet()
	p = &parser{
		DupMap: make(DupMap),
		intDupMap: make(DupMap),
	}
	if dups != nil{
		p.DupMap = dups
	}
	p.init(fset, "none", []byte(line), AllErrors)
	return p, nil
}

func (p *parser) parseDefinition() *ast.TypeSpec {
	if p.trace {
		defer un(trace(p, "Definition"))
	}
	p.openScope()
	defer p.closeScope()
	name := p.parseStructName()
	if name == nil {
		p.error(p.pos, "struct missing name")
		return nil
	}
	S := &ast.TypeSpec{
		Name: name,
		Type: &ast.StructType{Fields: &ast.FieldList{List: make([]*ast.Field, 0)}},
	}
	fp := S.Type.(*ast.StructType).Fields
	for p.tok != token.SEMICOLON && p.tok != token.EOF {
		f, i := p.parseWireField()

		fp.List = append(fp.List, f)
		TInfo.Add(S, f, i)
	}
	if fp.List == nil {
		p.error(p.pos, "empty field list")
		return nil
	}
	p.expectSemi()

	return S
}

func (p *parser) parseWireField() (*ast.Field, *Info) {
	if p.trace {
		defer un(trace(p, "WireField"))
	}
	name := p.parseIdent()
	p.expect(token.LBRACK)
	p.exprLev++
	defer func() { p.exprLev-- }()

	var flag WidthFlag
	width := p.parseWireWidth()
	if width != nil {
		switch width.(type) {
		case *ast.BasicLit:
			flag |= WidthLit
		case *ast.Ident, *ast.BinaryExpr:
			flag |= WidthVar
		default:
			panic("parseWireField " + fmt.Sprintf("%T", width))
		}
	}

	typ := p.parseWireType()
	if width == nil && typ == nil {
		p.error(p.pos, "width and type cannot both be empty")
		return nil, nil
	}
	if typ == nil {
		// check if 1,2,4,8, otherwise []byte
		if p.trace {
			un(trace(p, "TypeFromWidth"))
		}
		t, err := TypeFromWidth(width)
		if err != nil {
			p.error(p.pos, err.Error())
			return nil, nil
		}
		typ = t
	}
	if typ == nil {
		p.error(p.pos, "cant determine type")
		return nil, nil
	}
	endian := p.parseWireEndian()
	p.expect(token.RBRACK)
	return &ast.Field{Names: []*ast.Ident{name}, Type: typ},
		&Info{Width: width, Endian: endian, Flag: flag}
}

func (p *parser) tryConsumeComma() bool {
	if p.trace {
		defer un(trace(p, "tryConsumeComma"))
	}
	if p.tok == token.COMMA {
		p.next()
		return true
	}
	return false
}

func (p *parser) parseWireWidth() (x ast.Expr) {
	if p.trace {
		defer un(trace(p, "WireWidth"))
	}
	if p.tok != token.COMMA {
		x = p.parseExpr(false)
	}
	p.tryConsumeComma()
	return x
}

func (p *parser) parseWireType() (x ast.Expr) {
	if p.trace {
		defer un(trace(p, "WireType"))
	}
	defer p.tryConsumeComma()
	if p.tok == token.COMMA || p.tok == token.RBRACK {
		return nil
	}
	return p.tryIdentOrInt()
}

func (p *parser) parseWireEndian() binary.ByteOrder {
	if p.trace {
		defer un(trace(p, "WireEndian"))
	}
	defer p.tryConsumeComma()
	if p.tok == token.COMMA || p.tok == token.RBRACK {
		return nil
	}
	x := p.parseIdent()
	switch {
	case x == nil:
		p.error(p.pos, fmt.Sprintf("endian set to empty string"))
		return nil
	case x.Name == "BE":
		return binary.BigEndian
	case x.Name == "LE":
		return binary.LittleEndian
	case x.Name == "":
		return binary.LittleEndian
	}
	p.error(p.pos, fmt.Sprintf(`endian must be "LE" or "BE got %s"`, x.Name))
	return nil
}

// next advances to the next token.
func (p *parser) next() {
	p.next0()
	if p.errors.Err() != nil {
		panic(p.errors.Err())
	}
	if p.trace && p.pos.IsValid() {
		s := p.tok.String()
		switch {
		case p.tok.IsLiteral():
			p.printTrace(s, p.lit)
		case p.tok.IsOperator(), p.tok.IsKeyword():
			p.printTrace("\"" + s + "\"")
		default:
			p.printTrace(s)
		}
	}
}

// If the result is an identifier, it is not resolved.
func (p *parser) parseTypeName() ast.Expr {
	if p.trace {
		defer un(trace(p, "TypeName"))
	}
	ident := p.parseIdent()
	// don't resolve ident yet - it may be a parameter or field name
	if p.tok == token.PERIOD {
		// ident is a package name
		p.next()
		//p.resolve(ident)
		sel := p.parseIdent()
		return &ast.SelectorExpr{X: ident, Sel: sel}
	}
	return ident
}

func (p *parser) parseStructName() *ast.Ident {
	if p.trace {
		defer un(trace(p, "StructName"))
	}
	pos := p.pos
	name := "_"
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else {
		p.expect(token.IDENT) // use expect() error handling
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

func (p *parser) parseIdent() *ast.Ident {
	if p.trace {
		defer un(trace(p, "Ident"))
	}
	pos := p.pos
	name := "_"
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else if p.tok == token.STRING {
		// allow string literals as "identifiers"

	} else {
		p.expect(token.IDENT) // use expect() error handling
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

// If the result is an identifier, it is not resolved.
func (p *parser) tryIdentOrLit() ast.Node {
	switch p.tok {
	case token.IDENT:
		return p.parseTypeName()
	case token.STRING:
		return p.parseRHS()
	}
	// no type found
	return nil
}

func (p *parser) tryIdentOrInt() ast.Expr {
	switch p.tok {
	case token.IDENT:
		return p.parseIdent()
	case token.INT:
		return p.parseRHS()
	case token.LBRACK:
		return p.parseArrayType()
	case token.MUL:
		return p.parsePointerType()
	case token.STRING:
		return p.parseRHS()
	}
	// no type found
	return nil
}
func (p *parser) parsePointerType() *ast.StarExpr {
	if p.trace {
		defer un(trace(p, "PointerType"))
	}
	star := p.expect(token.MUL)
	base := p.parseType()
	return &ast.StarExpr{Star: star, X: base}
}
func (p *parser) parseArrayType() ast.Expr {
	if p.trace {
		defer un(trace(p, "ArrayType"))
	}

	lbrack := p.expect(token.LBRACK)
	p.exprLev++
	var len ast.Expr
	// always permit ellipsis for more fault-tolerant parsing
	if p.tok == token.ELLIPSIS {
		len = &ast.Ellipsis{Ellipsis: p.pos}
		p.next()
	} else if p.tok != token.RBRACK {
		len = p.parseRHS()
	}
	p.exprLev--
	p.expect(token.RBRACK)
	elt := p.parseType()

	return &ast.ArrayType{Lbrack: lbrack, Len: len, Elt: elt}
}
func (p *parser) parseType() ast.Expr {
	if p.trace {
		defer un(trace(p, "Type"))
	}
	typ := p.tryType()
	if typ == nil {
		pos := p.pos
		p.errorExpected(pos, "type")
		p.next() // make progress
		return &ast.BadExpr{From: pos, To: p.pos}
	}
	return typ
}
func (p *parser) tryType() ast.Expr {
	typ := p.tryIdentOrType()
	return typ
}

// If the result is an identifier, it is not resolved.
func (p *parser) tryIdentOrType() ast.Expr {
	switch p.tok {
	case token.IDENT:
		return p.parseTypeName()
	case token.LBRACK:
		return p.parseArrayType()
	case token.MUL:
		return p.parsePointerType()
	}
	// no type found
	return nil
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseBinaryExpr(lhs bool, prec1 int) ast.Expr {
	if p.trace {
		defer un(trace(p, "BinaryExpr"))
	}

	x := p.parseUnaryExpr(lhs)
	for _, prec := p.tokPrec(); prec >= prec1; prec-- {
		for {
			op, oprec := p.tokPrec()
			if oprec != prec {
				break
			}
			pos := p.expect(op)
			if lhs {
				lhs = false
			}
			y := p.parseBinaryExpr(false, prec+1)
			x = &ast.BinaryExpr{X: p.checkExpr(x), OpPos: pos, Op: op, Y: p.checkExpr(y)}
		}
	}

	return x
}
func (p *parser) tokPrec() (token.Token, int) {
	tok := p.tok
	if p.inRHS && tok == token.ASSIGN {
		tok = token.EQL
	}
	return tok, tok.Precedence()
}

// A bailout panic is raised to indicate early termination.
type bailout struct{}

func (p *parser) init(fset *token.FileSet, filename string, src []byte, mode Mode) {
	p.file = fset.AddFile(filename, -1, len(src))
	var m scanner.Mode
	if mode&ParseComments != 0 {
		m = scanner.ScanComments
	}
	eh := func(pos token.Position, msg string) { p.errors.Add(pos, msg) }
	p.scanner.Init(p.file, src, eh, m)

	p.mode = mode
	p.trace = mode&Trace != 0 // for convenience (p.trace is used frequently)
	p.next()
}

func (p *parser) printTrace(a ...interface{}) {
	const dots = ". . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . "
	const n = len(dots)
	pos := p.file.Position(p.pos)
	fmt.Printf("%5d:%3d: ", pos.Line, pos.Column)
	i := 2 * p.indent
	for i > n {
		fmt.Print(dots)
		i -= n
	}
	// i <= n
	fmt.Print(dots[0:i])
	fmt.Println(a...)
}

func (p *parser) next0() {
	// Because of one-token look-ahead, print the previous token
	// when tracing as it provides a more readable output. The
	// very first token (!p.pos.IsValid()) is not initialized
	// (it is token.ILLEGAL), so don't print it .
	if p.trace && p.pos.IsValid() {
		s := p.tok.String()
		switch {
		case p.tok.IsLiteral():
			p.printTrace(s, p.lit)
		case p.tok.IsOperator(), p.tok.IsKeyword():
			p.printTrace("\"" + s + "\"")
		default:
			p.printTrace(s)
		}
	}

	p.pos, p.tok, p.lit = p.scanner.Scan()
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseUnaryExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "UnaryExpr"))
	}

	switch p.tok {
	case token.ADD, token.SUB, token.NOT, token.XOR, token.AND:
		pos, op := p.pos, p.tok
		p.next()
		x := p.parseUnaryExpr(false)
		return &ast.UnaryExpr{OpPos: pos, Op: op, X: p.checkExpr(x)}

	case token.ARROW:
		// channel type or receive expression
		arrow := p.pos
		p.next()
		x := p.parseUnaryExpr(false)

		// determine which case we have
		if typ, ok := x.(*ast.ChanType); ok {
			// (<-type)

			// re-associate position info and <-
			dir := ast.SEND
			for ok && dir == ast.SEND {
				if typ.Dir == ast.RECV {
					// error: (<-type) is (<-(<-chan T))
					p.errorExpected(typ.Arrow, "'chan'")
				}
				arrow, typ.Begin, typ.Arrow = typ.Arrow, arrow, arrow
				dir, typ.Dir = typ.Dir, ast.RECV
				typ, ok = typ.Value.(*ast.ChanType)
			}
			if dir == ast.SEND {
				p.errorExpected(arrow, "channel type")
			}

			return x
		}

		// <-(expr)
		return &ast.UnaryExpr{OpPos: arrow, Op: token.ARROW, X: p.checkExpr(x)}

	case token.MUL:
		// pointer type or unary "*" expression
		pos := p.pos
		p.next()
		x := p.parseUnaryExpr(false)
		return &ast.StarExpr{Star: pos, X: p.checkExprOrType(x)}
	}

	return p.parsePrimaryExpr(lhs)
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parsePrimaryExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "PrimaryExpr"))
	}

	x := p.parseOperand(lhs)
L:
	for {
		switch p.tok {
		case token.PERIOD:
			p.next()
			if lhs {
			}
			switch p.tok {
			case token.IDENT:
				x = p.checkExprOrType(x)
			default:
				pos := p.pos
				p.errorExpected(pos, "selector or type assertion")
				p.next() // make progress
				sel := &ast.Ident{NamePos: pos, Name: "_"}
				x = &ast.SelectorExpr{X: x, Sel: sel}
			}
		default:
			break L
		}
		lhs = false // no need to try to resolve again
	}

	return x
}

// checkExprOrType checks that x is an expression or a type
// (and not a raw type such as [...]T).
//
func (p *parser) checkExprOrType(x ast.Expr) ast.Expr {
	switch t := unparen(x).(type) {
	case *ast.ParenExpr:
		panic("unreachable")
	case *ast.UnaryExpr:
	case *ast.ArrayType:
		if len, isEllipsis := t.Len.(*ast.Ellipsis); isEllipsis {
			p.error(len.Pos(), "expected array length, found '...'")
			x = &ast.BadExpr{From: x.Pos()}
		}
	}

	// all other nodes are expressions or types
	return x
}

// parseOperand may return an expression or a raw type (incl. array
// types of the form [...]T. Callers must verify the result.
// If lhs is set and the result is an identifier, it is not resolved.
//
func (p *parser) parseOperand(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "Operand"))
	}

	switch p.tok {
	case token.IDENT:
		x := p.parseIdent()
		if !lhs {
		}
		return x

	case token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING:
		x := &ast.BasicLit{ValuePos: p.pos, Kind: p.tok, Value: p.lit}
		p.next()
		return x
	}

	// we have an error
	pos := p.pos
	p.errorExpected(pos, "operand")
	return &ast.BadExpr{From: pos, To: p.pos}
}
func (p *parser) errorExpected(pos token.Pos, msg string) {
	msg = "expected " + msg
	if pos == p.pos {
		// the error happened at the current position;
		// make the error message more specific
		if p.tok == token.SEMICOLON && p.lit == "\n" {
			msg += ", found newline"
		} else {
			msg += ", found '" + p.tok.String() + "'"
			if p.tok.IsLiteral() {
				msg += " " + p.lit
			}
		}
	}
	p.error(pos, msg)
}

func (p *parser) error(pos token.Pos, msg string) {
	epos := p.file.Position(pos)

	// If AllErrors is not set, discard errors reported on the same line
	// as the last recorded error and stop parsing if there are more than
	// 10 errors.
	if p.mode&AllErrors == 0 {
		n := len(p.errors)
		if n > 0 && p.errors[n-1].Pos.Line == epos.Line {
			return // discard - likely a spurious error
		}
		if n > 10 {
			panic(bailout{})
		}
	}

	p.errors.Add(epos, msg)
}

func (p *parser) parseRHS() ast.Expr {
	old := p.inRHS
	p.inRHS = true
	x := p.parseExpr(false)
	p.inRHS = old
	return x
}

// If x is of the form (T), unparen returns unparen(T), otherwise it returns x.
func unparen(x ast.Expr) ast.Expr {
	if p, isParen := x.(*ast.ParenExpr); isParen {
		x = unparen(p.X)
	}
	return x
}

// If lhs is set and the result is an identifier, it is not resolved.
// The result may be a type or even a raw type ([...]int). Callers must
// check the result (using checkExpr or checkExprOrType), depending on
// context.
func (p *parser) parseExpr(lhs bool) ast.Expr {
	if p.trace {
		defer un(trace(p, "Expression"))
	}
	return p.parseBinaryExpr(lhs, token.LowestPrec+1)
}

// checkExpr checks that x is an expression (and not a type).
func (p *parser) checkExpr(x ast.Expr) ast.Expr {
	switch unparen(x).(type) {
	case *ast.BadExpr:
	case *ast.Ident:
	case *ast.BasicLit:
	case *ast.FuncLit:
	case *ast.CompositeLit:
	case *ast.ParenExpr:
		panic("unreachable")
	case *ast.SelectorExpr:
	case *ast.IndexExpr:
	case *ast.SliceExpr:
	case *ast.TypeAssertExpr:
		// If t.Type == nil we have a type assertion of the form
		// y.(type), which is only allowed in type switch expressions.
		// It's hard to exclude those but for the case where we are in
		// a type switch. Instead be lenient and test this in the type
		// checker.
	case *ast.CallExpr:
	case *ast.StarExpr:
	case *ast.UnaryExpr:
	case *ast.BinaryExpr:
	default:
		// all other nodes are not proper expressions
		p.errorExpected(x.Pos(), "expression")
		x = &ast.BadExpr{From: x.Pos()}
	}
	return x
}
func (p *parser) expectSemi() {
	if p.tok == token.SEMICOLON {
		p.next()
	} else {
		p.errorExpected(p.pos, "';'")
	}
}

func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.pos
	if p.tok != tok {
		p.errorExpected(pos, "'"+tok.String()+"'")
	}
	p.next() // make progress
	return pos
}
func trace(p *parser, msg string) *parser {
	p.printTrace(msg, "(")
	p.indent++
	return p
}

// Usage pattern: defer un(trace(p, "..."))
func un(p *parser) {
	p.indent--
	p.printTrace(")")
}

func (p *parser) checkOldStruct(name string, old ast.TypeSpec) {
	new := TInfo.NamedStruct(name)
	for i, of := range old.Type.(*ast.StructType).Fields.List {
		nf := new.Type.(*ast.StructType).Fields.List[i]
		oname, nname := of.Names[0].Name, nf.Names[0].Name
		if oname != nname {
			p.error(p.pos, fmt.Sprintf("struct field name changed: %s", name))
		}
	}
}

func (p *parser) openScope() {
	p.topScope = ast.NewScope(p.topScope)
}
func (p *parser) closeScope() {
	p.topScope = p.topScope.Outer
}
