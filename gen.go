package wire9

import (
	"fmt"
	"go/ast"
	"io"
	"text/template"

	"log"
)

var (
	eStruct      = template.Must(template.New("tStruct").Funcs(funcMap).Parse(tStruct))
	eWriteBinary = template.Must(template.New("tWriteBinary").Funcs(funcMap).Parse(tWriteBinary))
	eReadBinary  = template.Must(template.New("tReadBinary").Funcs(funcMap).Parse(tReadBinary))
)

var Flags = map[string]bool{}

func WasSet(s string) bool {
	_, ok := Flags[s]
	Flags[s] = true
	return ok
}

func (p *parser) genTypes(w io.Writer, exprs ...*ast.TypeSpec) (err error) {
	if !WasSet("tWriteString") {
		w.Write([]byte(tWriteString))
	}
	for _, e := range exprs {
		if id := p.DupMap[Dup{e.Name.Name, "StructType"}]; id != nil {
			log.Printf("gen: skip already-defined struct: %s\n", id.Name)
			fmt.Fprintf(w, "// type %s struct { //defined in other file", id.Name)
			continue
		}
		if err = eStruct.Execute(w, e); err != nil {
			return
		}
	}
	return nil
}

func (p *parser) genFuncs(w io.Writer, exprs ...*ast.TypeSpec) (err error) {
	for _, e := range exprs {
		if id := p.DupMap[Dup{e.Name.Name, "ReadBinary"}]; id != nil {
			log.Printf("gen: skip already-defined ReadBinary method for: %s\n", id.Name)
			fmt.Fprintf(w, "// func (z %s) ReadBinary { // defined in other file}\n", id.Name)
		} else {
			if err = eReadBinary.Execute(w, e); err != nil {
				return
			}
		}
		if id := p.DupMap[Dup{e.Name.Name, "WriteBinary"}]; id != nil {
			log.Printf("gen: skip already-defined WriteBinary method for: %s\n", id.Name)
			fmt.Fprintf(w, "// func (z %s) WriteBinary { //  defined in other file\n", id.Name)
		} else {
			if err = eWriteBinary.Execute(w, e); err != nil {
				return
			}
		}
	}
	return nil
}

//
// Template functions

var funcMap = template.FuncMap{
	"bailout":      func() (int, error) { return 1, fmt.Errorf("bailout") },
	"array":        Array,
	"bytearray":    ByteArray,
	"byteslice":    ByteSlice,
	"coherent":     Coherent,
	"custom":       Custom,
	"customslice":  CustomSlice,
	"endian":       Endian,
	"numeric":      Numeric,
	"slice":        Slice,
	"string":       String,
	"typeof":       TypeString,
	"fields":       func(s ast.TypeSpec) []*ast.Field { return s.Type.(*ast.StructType).Fields.List },
	"looped":       func(f ast.Expr) bool { return (Nesting.Inc(!ByteSlice(f) && !ByteArray(f) && (Slice(f) || Array(f)))) },
	"unlooped":     func(f ast.Expr) bool { return (Nesting.Dec(!ByteSlice(f) && !ByteArray(f) && (Slice(f) || Array(f)))) },
	"initial":      func(f ast.Expr) bool { return Slice(f) || Array(f) },
	"alloc":        func(f ast.Expr) bool { return Slice(f) || Array(f) },
	"maketmp":      func(f ast.Expr) bool { return Literal(f) },
	"normal":       func(f ast.Expr) bool { return ByteSlice(f) },
	"wired":        func(f ast.Expr) bool { return !Slice(f) && !Array(f) },
	"binary":       func(f ast.Expr) bool { return Numeric(f) },
	"width":        WidthOf,
	"literal":      Literal,
	"declaredname": func(f *ast.Field) string { return f.Names[0].Name },
	"name": func(f *ast.Field) (n string) {
		n = f.Names[0].Name
		if Nesting > 0 {
			n = fmt.Sprintf("%s[i]", n)
		}
		return
	},
}

//
// Templates

const tWriteString = `
	func writestring(w io.Writer, s string, must int) (err error) {
		data := []byte(s)
		switch l := len(data); {
		case l > must:
			_, err = w.Write(data[:must])
		case l < must:
			_, err = w.Write(data[:l])
			if err != nil{
				return err
			}
			underflow := must - l
			_, err = w.Write(bytes.Repeat([]byte{0x00}, underflow))
		default:
			_, err = w.Write(data[:l])
		}
		return err
	}
	
	func ioErr(name, kind string, ac, ex int) error{
		return fmt.Errorf("%s: short %s: %d/%d", name, kind, ac, ex)
	}
`

const tStruct = `
{{ with $s := . }}
{{ with $nm := $s.Name | printf "%s" }}
{{ with $fl :=  $s | fields}}
type {{ $nm }} struct{
	{{- range $i, $v := $fl -}}
		{{- if $v.Type | literal }}
			{{ "todo" |  printf "// %s" }}{{- else -}}
			{{$v | name }} {{ $v.Type | typeof }}
		{{- end }}
	{{ end}}
{{- end}}{{- end}}{{- end}}
}
`
const tReadBinary = `
{{ with $st := . }}
{{ with $nm := .Name | printf "%s" }}
	func (z *{{$nm}}) ReadBinary(r io.Reader) (err error) {
		defer func() { recover() }()
		if z == nil {return fmt.Errorf("ReadBinary: z nil") };
		{{- range $i, $f := $st | fields}}
			{{with $nm  := $f | name}}{{with $typ := $f | typeof }}
				{
				{{- if $f.Type | looped }}
                	z.{{$f | declaredname }} = make({{$typ}}, {{ ((width $st $f)) }})
					for i := 0; i < {{ ((width $st $f)) }}; i++ {
                {{else}}
					{{ if $f.Type | alloc    }}  z.{{$f | name}} = make([]byte, {{ ((width $st $f)) }}) {{else}}
					{{ if $f.Type | initial  }}    {{$f | name}} = {{$f | typeof}}{} {{else}}
					{{ if $f.Type | maketmp  }}  tmp := make([]byte, {{ ((width $st $f)) }}) {{end}}{{end}}{{end}}
				{{end}}

				{{ if $f.Type | normal	}}      if n, err := r.Read(z.{{$f | name}});       err != nil || n != {{ ((width $st $f)) }}  {return err}{{else}}
				{{ if $f.Type | customslice }} if    err := z.{{$f | name}}.ReadBinary(r); err != nil { return err } {{else}}
				{{ if $f.Type | binary	}}      if err :=    binary.Read(r, {{$f | endian }}, &z.{{$f | name}}); err != nil { return err } {{else}}
				{{ if $f.Type | literal	}}  if n, err := r.Read(tmp); err != nil || bytes.Compare(tmp, []byte({{$f | name}})) != 0 {
						if err != nil { return return fmt.Errorf("z.%x: read %x instead", []byte({{$f | name}}, tmp)};
						return err;
					}{{else}}
				{{ if $f.Type | wired }} if    err := z.{{$f | name}}.ReadBinary(r); err != nil { return err } ;{{- end}}{{- end}}{{- end}}{{- end}}{{- end}}
			{{- if $f.Type | unlooped }} } {{end}}{{end}}
		{{- end}}
				}{{end}}
		return nil
	}
{{end}}
{{end}}
`
const tWriteBinary = `
{{ with $st := . }}
{{ with $nm := .Name | printf "%s" }}
	func (z *{{$nm}}) WriteBinary(w io.Writer) (err error) {
		defer func() { recover() }()
		{{- range $i, $f := $st | fields}}
			{{with $nm  := $f | name}}{{with $typ := $f | typeof }}
				{
				{{- if $f.Type | looped }}
                  z.{{$f | declaredname }} = make({{$typ}}, {{ ((width $st $f)) }})
				  for i := 0; i < {{ ((width $st $f)) }}; i++ {
                {{else}}{{end}}

				{{- if $f.Type | normal  }} x := {{ ((width $st $f)) }}; if n, err := w.Write(z.{{$f | name}}[:x]); err != nil || n != x  {return err}  {{else}} 
				{{- if $f.Type | customslice   }} if err := z.{{$f | name}}.WriteBinary(w); err != nil { return err } {{else}}
				{{- if $f.Type | binary  }} if err := binary.Write(w, {{$f | endian }}, z.{{$f | name}}); err != nil { return err } {{else}}
				{{- if $f.Type | literal }} if n, err := w.Write([]byte({{$f | name}})); err != nil || n != len([]byte({{$f | name}})) {
						if err != nil {
							return return fmt.Errorf("z.%x: write %x instead", []byte({{$f | name}}, tmp)
						}
						return err
					}{{else}} 
				{{ if $f.Type | wired }} if    err := z.{{$f | name}}.WriteBinary(w); err != nil { return err } ;
				{{else}}{{ call bailout }}{{- end}}{{- end}}{{- end}}{{- end}}{{- end}}
			{{- if $f.Type | unlooped }} } {{end}}{{end}}
		{{- end}}
				}{{end}}
		return nil
	}
{{end}}
{{end}}
`
const ztWriteBinary = `
{{ with $st := . }}
{{ with $nm := .Name | printf "%s" }}
	func (z {{$nm}}) WriteBinary(w io.Writer) (err error) {
		defer func() { recover() }()
		{{ range $i, $f := $st | fields}}
			{{with $nm  := $f | name}}{{with $len :=  ((width $st $f)) }}{{with $typ := $f | typeof }}
			{{- if $f.Type | looped }} for i := 0; i < {{ ((width $st $f)) }}; i++ { {{ end }}
				{
				{{- if $f.Type | normal  }} x := {{ ((width $st $f)) }}; if n, err := w.Write(z.{{$f | name}}[:x]); err != nil || n != x  {return err}  {{else}} 
				{{- if $f.Type | customslice   }} if err := z.{{$f | name}}.WriteBinary(w); err != nil { return err } {{else}}
				{{- if $f.Type | binary  }} if err := binary.Write(w, {{$f | endian }}, z.{{$f | name}}); err != nil { return err } {{else}}
				{{- if $f.Type | literal }} if n, err := w.Write([]byte({{$f | name}})); err != nil || n != len([]byte({{$f | name}})) {
						if err != nil {
							return return fmt.Errorf("z.%x: write %x instead", []byte({{$f | name}}, tmp)
						}
						return err
					}{{else}} 
				{{ if $f.Type | wired }} if    err := z.{{$f | name}}.WriteBinary(w); err != nil { return err } ;{{- end}}{{- end}}{{- end}}{{- end}}{{- end}}
				}
			{{ if $f.Type | unlooped }} }{{ end }}
		{{end}}{{end}}{{end}}
{{end}}
		return nil
	}
{{end}}
{{end}}
`
