# wire9
Protocol Boilerplate Generator

# install
Build wire9/cmd

# definitions
wire definitions are defined with a comment starting with '//wire9'
```
//wire9 TypeName MemberName[Width,Type,Endian] ...

//wire9 Dot1 q0[8] q1[8]
//wire9 Dot2 q0[8,int64] q1[,int64]
//wire9 Dot3 q0[8,int64,BE] q1[8,,BE]
```

At least one Width or Type must be defined per member. Endianness defaults to LE (little-endian).

# Example 1
Conformant type Length prefixed string. Members preceeding other members
in wire definitions may indicate the width of the members it preceeds.
```
package main

// bstr n[4] data[n]

go:generate wire9 -f main_wire9.go .

func main(){
  bs := &bstr{}
  bs.ReadBinary(os.Stdin) 
  // Reads in 0x0500000041
  // Final value
  // &bstr{n: 5, data: "A"}
}

```

# Example 2: Nested conformant types
A batch request (two conformant types)
```
package main

// bstr  n[4] data[n]
// batch n[4] strings[n,[]bstr]

go:generate wire9 -f main_wire9.go .

func main(){
  bs := &batch{}
  bs.ReadBinary(os.Stdin) 
  // Reads in: 0x03000000010000004101000000410100000041
  // Final value
  // &batch{
      n: 3,
      data: []bstr{
         bstr{n: 1, data: "A"}
         bstr{n: 1, data: "B"}
         bstr{n: 1, data: "C"}
      }
}

```
