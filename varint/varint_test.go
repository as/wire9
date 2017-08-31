package varint

import "testing"
import "bytes"

func testRead(t *testing.T, name string, want int, in string) {
	buf := bytes.NewBufferString(in)
	var v = new(V)
	v.ReadBinary(buf)
	have := *v
	if V(want) != have {
		t.Logf("%s: \n\thave=%d\n\twant=%d\n", name, have, want)
		t.Fail()
	}
}

func testWrite(t *testing.T, name string, in int, want string) {
	buf := new(bytes.Buffer)
	v := V(in)
	v.WriteBinary(buf)
	have := string(buf.Bytes())
	w, h := []byte(want), []byte(have)
	if bytes.Compare(w, h) != 0 {
		t.Logf("%s: \n\thave=% x\n\twant=% x\n", name, have, want)
		t.Fail()
	}
}

func TestReadBarrier(t *testing.T) {
	varint := "\x80\x80\x80\x80\x01"
	wall := "readbarrier"
	buf := bytes.NewBufferString(varint + wall)
	var v = new(V)
	v.ReadBinary(buf)
	if have := string(buf.Bytes()); have != "readbarrier" {
		t.Logf("readbarrier: \n\thave=%s\n\twant=readbarrier\n", have)
		t.Fail()
	}
}

func TestWriteBinary(t *testing.T) {
	testWrite(t, "zero", 0, "\x00")
	testWrite(t, "one", 1, "\x01")
	testWrite(t, "128^1-1", 127, "\x7f")
	testWrite(t, "128^1  ", 128, "\x80\x01")
	testWrite(t, "128^1+1", 129, "\x81\x01")
	testWrite(t, "128^2-1", 128*128-1, "\xff\x7f")
	testWrite(t, "128^2  ", 128*128, "\x80\x80\x01")
	testWrite(t, "128^2+1", 128*128+1, "\x81\x80\x01")
	testWrite(t, "128^3 ", 128*128*128, "\x80\x80\x80\x01")
	testWrite(t, "128^4", 128*128*128*128, "\x80\x80\x80\x80\x01")
	testWrite(t, "128^8", 128*128*128*128*128*128*128*128, "\x80\x80\x80\x80\x80\x80\x80\x80\x01")
}

func TestReadBinary(t *testing.T) {
	testRead(t, "zero", 0, "\x00")
	testRead(t, "one", 1, "\x01")
	testRead(t, "128^1-1", 127, "\x7f")
	testRead(t, "128^1+0", 128, "\x80\x01")
	testRead(t, "128^1+1", 129, "\x81\x01")
	testRead(t, "128^2-1", 128*128-1, "\xff\x7f")
	testRead(t, "128^2+0", 128*128, "\x80\x80\x01")
	testRead(t, "128^2+1", 128*128+1, "\x81\x80\x01")
	testRead(t, "128^3+0 ", 128*128*128, "\x80\x80\x80\x01")
	testRead(t, "128^4+0", 128*128*128*128, "\x80\x80\x80\x80\x01")
	testRead(t, "128^8+0", 128*128*128*128*128*128*128*128, "\x80\x80\x80\x80\x80\x80\x80\x80\x01")
}
