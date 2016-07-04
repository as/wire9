package main

import (
	"bytes"
	"fmt"
	"log"
	"io"
)

//wire9 Bstr n[2] data[n]

func main(){
	buf := new(bytes.Buffer)
	tx := Bstr{5, "hello"}
	tx.WriteBinary(buf)

	fmt.Printf("% #x\n", buf.Bytes())
	
	rx := &Bstr{}
	rx.ReadBinary(buf)
	fmt.Printf("%#v", rx)
}
