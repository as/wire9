package wire9

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"strings"
)

var TInfo *TypeInfo

func init() {
	TInfo = NewTypeInfo()
}

// WidthFlag describes a width associated with a Go type.
type WidthFlag int

// WidthFlag values
const (
	WidthLit WidthFlag = 1 << iota
	WidthVar
	WidthBad
)

// On returns true is bits are set
func (w WidthFlag) On(bits WidthFlag) bool {
	return w&bits == w
}

// Package is a box for AST, wire9 and package type information.
type Package struct {
	Info     *types.Info
	Path     string
	Files    []string
	ASTFiles []*ast.File
	Data     []byte
	Fset     *token.FileSet
	Pkg      *types.Package
	DupMap DupMap
}

// Source files. TODO: Revise
type Source struct {
	Files   []*File
	fs      *token.FileSet
	p       *parser
	Structs []*ast.TypeSpec
	DupMap
}

// File struct. TODO: Revise
type File struct {
	Name    string
	Structs []*ast.TypeSpec
}

// TypeInfo collects Info structures for every
// wire definition (struct)
type TypeInfo struct {
	m        map[string]map[string]*Info
	Nstructs map[string]ast.TypeSpec
}

// Info holds type information for a field parsed
// with wire9
type Info struct {
	Width        ast.Expr
	Endian       binary.ByteOrder
	Flag         WidthFlag
	FromGoSource bool
}

// OpenPackage opens the package at path. It returns a partialy-initialized Package
// containing initialized ASTFiles, Fset, and Files.
func OpenPackage(path string, dowires bool) (pkg *Package, err error) {
	pkg = &Package{
		Fset: token.NewFileSet(),
		Info: &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)},
		Path: path,
		DupMap: make(DupMap),
	}
	okfile := func(fi os.FileInfo) bool {
		if !dowires && WireFile(fi) {
			return false
		}
		pkg.Files = append(pkg.Files, fmt.Sprintf("%s%c%s", path, os.PathSeparator, fi.Name()))
		return true
	}
	pkgmap, err := goparser.ParseDir(pkg.Fset, path, okfile, goparser.AllErrors | goparser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, v := range pkgmap {
		for _, f := range v.Files {
			pkg.ASTFiles = append(pkg.ASTFiles, f)
			pkg.DupMap.Merge(DupFind(f))
		}
	}
	return pkg, nil
}

// FromFiles produces wire9 structures and functions by reading wire
// definitions from files. Dofmt controls gofmt operation.
func FromFiles(files []string, dups DupMap, dofmt bool) ([]byte, error) {
	const Banner = `
	package main
	
	import (
		"encoding/binary"
		"bytes"
		"io"
		"fmt"
	)
	`
	var buf bytes.Buffer
	fmt.Fprint(&buf, Banner)

	src, err := ParseFiles(files, dups)
	if err != nil {
		return nil, err
	}
	if err = src.Generate(&buf); err != nil {
		return nil, err
	}
	data := Clean(buf.Bytes())
	if !dofmt {
		data, err = format.Source(data)
		if err != nil {
			return nil, fmt.Errorf("gofmt: %s", err)
		}
	}
	return data, nil
}

// FromPackage produces wire9 structures and functions via from a Package
// opened with OpenPackage.
func FromPackage(pkg *Package, dofmt bool) (wire *Package, err error) {
	data, err := FromFiles(pkg.Files, pkg.DupMap, dofmt)
	if err != nil {
		return nil, err
	}
	wire = &Package{
		Fset:  token.NewFileSet(),
		Files: pkg.Files,
		Data:  data,
		Path:  pkg.Path,
		Info:  &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)},
	}
	file, err := goparser.ParseFile(wire.Fset, "", wire.Data, goparser.AllErrors | goparser.ParseComments)
	if err != nil {
		return nil, err
	}
	wire.ASTFiles = []*ast.File{file}
	return wire, nil
}

// Eval parses and processes a line of input as a wire definition. The
// definition must start with a double slash and follow the format
// given in the package description comment.
func (src *Source) Eval(line string, dups DupMap) (st *ast.TypeSpec, err error) {
	fmt.Println("eval", line)
	src.fs = new(token.FileSet)
	if src.p, err = NewParser(src.fs, line, dups); err != nil {
		return
	}
	x := src.p.parseDefinition()
	return x, src.p.errors.Err()
}

// Parse parses the named file and produces a list of type specifications
func (src *Source) Parse(name string) ([]*ast.TypeSpec, error) {
	fd, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	s := bufio.NewScanner(fd)
	file := &File{
		Name: name,
	}
	for s.Scan() {
		t := s.Text()
		if !strings.HasPrefix(t, "//wire9") {
			continue
		}
		list, err := src.Eval(t, src.DupMap)
		if err != nil {
			return nil, err
		}
		file.Structs = append(file.Structs, list)
	}
	src.Files = append(src.Files, file)
	return src.Structs, err
}

// NewTypeInfo returns an initialized *TypeInfo
func NewTypeInfo() *TypeInfo {
	return &TypeInfo{
		m:        make(map[string]map[string]*Info),
		Nstructs: make(map[string]ast.TypeSpec),
	}
}

// Add associates a named struct field with an *Info structure.
func (t *TypeInfo) Add(nstruct *ast.TypeSpec, field *ast.Field, i *Info) {
	if nstruct == nil {
		return
	}
	var f, n string
	n = nstruct.Name.Name

	// Allow an empty string to mark a struct as declared
	if field != nil {
		f = field.Names[0].Name
	}
	t.Nstructs[n] = *nstruct

	if _, ok := t.m[n]; !ok {
		t.m[n] = make(map[string]*Info)
	}
	t.m[n][f] = i
}

// Get returns an *Info field for a named struct field.
func (t *TypeInfo) Get(nstruct *ast.TypeSpec, field *ast.Field) (i *Info) {
	if nstruct == nil {
		return nil
	}
	var f, n string
	n = nstruct.Name.Name
	if field != nil {
		f = field.Names[0].Name
	}
	return t.m[n][f]
}

func (t *TypeInfo) NamedStruct(n string) ast.TypeSpec {
	return t.Nstructs[n]
}

// ParseLine parses and processes a line of input as a wire definition. TODO: Consolidate
func (src *Source) ParseLine(line string) (*ast.TypeSpec, error) {
	st, err := src.Eval(line, src.DupMap)
	if err != nil {
		return nil, err
	}
	src.Structs = append(src.Structs, st)
	return st, err
}

// ParseFiles parses files listed in fs and extracts all sys comments.
// It returns source files and their list of wire9 expressions
func ParseFiles(fs []string, dups DupMap) (src *Source, err error) {
	src = &Source{DupMap: dups}
	for _, f := range fs {
		if _, err := src.Parse(f); err != nil {
			return nil, err
		}
	}
	return src, nil
}

// WireFile returns true if file contains "_wire9". The file's
// underlying type must be string or os.FileInfo.
func WireFile(file interface{}) bool {
	var name string
	switch s := file.(type) {
	case string:
		name = s
	case os.FileInfo:
		name = s.Name()
	case interface{}:
		panic("WireFile: %s not string or os.FileInfo")
	}
	if strings.Contains(name, "_wire9.go") {
		return true
	}
	return false
}

// Check runs a type checker on the package using config.
func (p *Package) Check(conf *types.Config) (*types.Info, error) {
	var err error
	p.Pkg, err = conf.Check(p.Path, p.Fset, p.ASTFiles, p.Info)
	return p.Info, err
}

// Clean examines scopes in input src and removes unnecessary braces. It
// returns the clean copy. The src is assumed to be valid, error-free Go
// source.
func Clean(src []byte) []byte {
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	c := NewCleaner()
	ast.Walk(c, f)
	b := new(bytes.Buffer)
	printer.Fprint(b, fset, f)
	return b.Bytes()
}

// Generate outputs source file from a source set src.
func (src *Source) Generate(w io.Writer) error {
	for _, t := range src.Files {
		if err := src.p.genTypes(w, t.Structs...); err != nil {
			return err
		}
	}
	for _, t := range src.Files {
		if err := src.p.genFuncs(w, t.Structs...); err != nil {
			return err
		}
	}
	return nil
}
