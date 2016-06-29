/*
Package wire9 provides a boilerplate protocol generator. Wire9 reads
wire definitions from Go source and generates structures and functions
to transcieve these data over the wire. 

Wire Definitions:

A wire definition is a Go comment, it begins like this:

//wire9

The prefix is followed by a struct name and a variadic list of fields.

	//wire9 structname field⁰[width,type,endian] ... fieldⁿ[width,type,endian]

A field starts with its name. It will become the name of a struct field.

Next, a [bracket-enclosed], comma-seperated list of field options: width, type,
and endian.

	field⁰[width,type,endian]

Width is a field's width over the wire. This can be an integer constant,
a previously-defined numeric field, or empty.

Types specify a field's type.  This affects the type value in the
final struct field, numeric and fixed types may have empty widths.

The width and type are interpreted one of three ways:

	1. Width numeric literal
		A. Type is empty: If width is 1, 2, 4, or 8, type is byte, uint16,
           uint32, and uint64, respectively.  Otherwise type is []byte.

		B. Type is identifier: The type represents a fixed width struct, numeric value, or slice
           type. The width must match the fixed types or the number of expected slice elements.

	2. Width is identifier
		A. Type is empty: Type is implicitly []byte. The width represents the number
		   of bytes to expect in the slice. 
           
		B. Type is identifier: The type represents a slice type. The width is
		   the number of expected elements in the slice.
		
	3. Width is empty
		A. Type is a fixed-width struct, number, or implements the Wire interface.

	Examples:
	
	//wire9 Ex1A Time[8]        IP[4]        Port[2]         n[1]
	//wire9 Ex1B Time[8,uint64] IP[4,uint32] Port[2,uint16]  n[1,byte]
	
	//wire9 Ex2A n[1]  URL[n]
	//wire9 Ex2B n[1]  URL[n,[]byte]
	
	//wire9 Ex3A p[,image.Point]  size[,int64]  reply[,Ex2A]

Endian specifies byte-order (LE or BE), which stand for Little-Endian
and Big-Endian, respectively.  LE is the Default.  Used for numeric
types and slices of numeric types.

	//wire9 Git index[4,,BE] ...

Usage:

Wire9 understands dynamic width. This example defines four common length-prefixed
strings and then generates structures and functions.

	//wire9 Pstr  n[1] data[n]
	//wire9 Bstr  n[2] data[n]
	//wire9 Dstr  n[4] data[n]
	//wire9 Qstr  n[8] data[n]
	
The following construction builds on the last example.  The value of
the four 4-byte integers set the expected number of slice elements for
each corresponding length-prefixed string.

	//wire9 BurstRX np[4] nb[4] nd[4] nq[4] SP[np,[]Pstr] SB[nb,[]Bstr] SD[nd,[]Dstr] SQ[nq,[]Qstr]

Trivia:

The goal of this package is to save time when implementing custom protocols,
such as Microsoft RDP. The package's original purpose was to take the Plan 9
draw(3) manual page and generate Go for parsing the messages written
to the data file.

*/
package wire9
