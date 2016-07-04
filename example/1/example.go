package main

import (
	"bytes"
	"fmt"
	"log"
	"io"
)

//go:generate wire9 -f example_wire9.go ./

// The wire9 structs will be generated along with their ReadBinary
// and WriteBinary methods. This will be stored in example_wire9.go as
// per the command line given to 'go generate' above.

// Good old-"friends" from win32
// lines beginning with '//wire9' are interpreted by the generator.

//wire9 Pstr   n[1] data[n]
//wire9 Bstr   n[2] data[n]
//wire9 Mestr  n[4] data[n]
//wire9 u64s   n[8]        data[n]
//wire9 i64s   n[8,int64]  data[n]
//wire9 BBEStr n[8,int64,LE] data[n]
//wire9 ApeStr n[2,uint16,BE] data[n,[]Pstr]

type Wire interface {
	ReadBinary(io.Reader) error
	WriteBinary(io.Writer) error
}

//
// main shows an example using the auto-generated Bstr, Pstr, and Mestr
// structs with their MarshalBinary/Unmarshalbinary methods.
//
func main() {
	names := []string{"Bstr", "Pstr", "Mestr", "ApeStr"}
	full := []Wire{
		&Bstr{5, []byte("Wire9")},
		&Pstr{5, []byte("Wire9")},
		&Mestr{5, []byte("Wire9")},
		&ApeStr{2, []Pstr{{5, []byte("HELLO")}, {4, []byte("WURL")}}},
	}
	empty := []Wire{
		&Bstr{0, nil},
		&Pstr{0, nil},
		&Mestr{0, nil},
		&ApeStr{0, nil},
	}
	for i, v := range full {
		data := new(bytes.Buffer)
		err := v.WriteBinary(data)
		ck(err)
		fmt.Printf("marshal %s into bytes: %v\n", names[i], data.Bytes())

		err = empty[i].ReadBinary(data)
		ck(err)
		fmt.Printf("unmarshal to empty %s: %v\n", names[i], empty[i])
		fmt.Println()
	}
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}


