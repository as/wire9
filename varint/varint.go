// Package varint provides the varint type V. V knows how to write
// and read its binary encoded representation from a reader or writer
package varint

import (
	"errors"
	"io"
)

var (
	MaxVLen16 = 3
	MaxVLen32 = 5
	MaxVLen64 = 10
)

// V is a varint that knows how to read and write its binary form.
// its in-memory representation is always uint64.
//
// After a call to ReadBinary, V can be converted to an unsigned
// integer. Likewise, setting V and calling WriteBinary writes
// V's varint value to the writer.
//
// This implementation does not handle signed values or zig-zag
// encoding.
type V uint64

var ErrOverflow = errors.New("varint: varint overflows a 64-bit integer")

// WriteBinary writes the varint to the underlying writer.
func (v V) WriteBinary(w io.Writer) (err error) {
	for err == nil {
		u := byte(v % 128)
		v /= 128
		if v > 0 {
			u |= 128
		}
		_, err = w.Write([]byte{u})
		if v <= 0 {
			break
		}
	}
	if err != nil || err != io.EOF {
		return err
	}
	return nil
}

// ReadBinary read a varint from the underlying reader. It does not
// read beyond the varint.
func (v *V) ReadBinary(r io.Reader) error {
	var b [1]byte
	m := int64(1)
	for n := 0; n < MaxVLen64; n++ {
		_, err := r.Read(b[:])
		if err != nil && err != io.EOF {
			return err
		}
		*v += V((int64(b[0]&127) * m))
		m *= 128
		if b[0]&128 == 0 {
			return nil
		}
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
	}
	return ErrOverflow
}
