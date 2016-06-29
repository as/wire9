package wire9

import (
	"bytes"
	"fmt"
	"testing"
)

func testparse(line string) error {
	b := new(bytes.Buffer)
	s := new(Source)
	if _, err := s.ParseLine(line); err != nil {
		return err
	}
	if err := s.Generate(b); err != nil {
		return err
	}
	fmt.Println(b.Bytes())
	return nil
}

func TestErrBasic(t *testing.T) {
	ck(t, "//wire9 S dstid[4] srcid[4] fontid[4] rpcmux[2] quid[8]\n")
	ck(t, "//wire9 T myint[2] otherint[2]\n")
	ck(t, "//wire9 Z myint[2] bytething[myint]\n")
}
func TestErrDynamic(t *testing.T) {
	ck(t, "//wire9 S dstid[4] srcid[4] fontid[4] rpcmux[2] quid[8]\n")
	ck(t, "//wire9 T myint[2] otherint[2]\n")
	ck(t, "//wire9 Z myint[2] bytething[myint]\n")
}
func TestErrPlan9Draw(t *testing.T) {
	ck(t, "//wire9 String data[11] z[1]\n")
	ck(t, "//wire9 DrawNew n[12,[]String]\n")
	ck(t, "//wire9 Space byte[1]\n")
	ck(t, "//wire9 Point X[4] Y[4]\n")
	ck(t, "//wire9 Rect  XMin[4] YMin[4] XMax[4] YMax[4]\n")
	ck(t, "//wire9 DrawA id[4] imageid[4]  fillid[4]  public[1]\n")
	ck(t, "//wire9 Drawb id[4] screenid[4] refresh[1] Chan[4] repl[1] r[16,Rect] clipr[16,Rect] color[4]\n")
	ck(t, "//wire9 Drawc dstid[4] repl[1] clipr[4*4]\n")
	ck(t, "//wire9 Drawd dstid[4] srcid[4] maskid[4] dstr[,Rect] srcp[,Point] maskp[,Point]\n")
	ck(t, "//wire9 DrawD debugon[1]\n")
	ck(t, "//wire9 Drawe dstid[4] srcid[4] c[2*4] a[4] b[4] thick[4] sp[2*4] alpha[4] phi[4]\n")
	ck(t, "//wire9 DrawE dstid[4] srcid[4] c[2*4] a[4] b[4] thick[4] sp[2*4] alpha[4] phi[4]\n")
	ck(t, "//wire9 Drawf id[4]\n")
	ck(t, "//wire9 DrawF id[4]\n")
	ck(t, "//wire9 Drawi id[4] n[4] ascent[1]\n")
	ck(t, "//wire9 Drawl cacheid[4] srcid[4] index[2] r[,Rect] sp[,Rect] left[1] width[1]\n")
	ck(t, "//wire9 DrawL dstid[4] p0[,Point] p1[,Point] e0[4] e1[4] thick[4] srcid[4] sp[,Point]\n")
	ck(t, "//wire9 DrawN id[4] in[1] j[1] name[j]\n")
	ck(t, "//wire9 Drawn id[4] j[1] name[j]\n")
	ck(t, "//wire9 Drawo id[4] min[8] scr[8]\n")
	ck(t, "//wire9 DrawO op[1]\n")
	ck(t, "//wire9 Drawp dstid[16] n[2] end0[16] end1[16] thick[16] srcid[16] sp[8] dp[13]\n")
	ck(t, "//wire9 DrawP dstid[16] n[2] wind[16] ignore[8] srcid[16] sp[8] dp[13]\n")
	ck(t, "//wire9 Drawr id[16] r[256]\n")
	ck(t, "//wire9 Draws dstid[16] srcid[16] fontid[16] p[8] clipr[256] sp[8] n[2] index[2]\n")
	ck(t, "//wire9 Drawx dstid[16] srcid[16] fontid[16] dp[8] clipr[256] sp[8] n[2] bgid[16] bp[8] index[2]\n")
	ck(t, "//wire9 DrawS id[4] Chan[4]\n")
	ck(t, "//wire9 Drawt top[1] n[2] id[16]\n")
	ck(t, "//wire9 Drawv id[4]\n")
	ck(t, "//wire9 Drawy id[16] r[256] buf[1]\n")
	ck(t, "//wire9 DrawY id[16] r[256] buf[1]\n")
}

func ck(t *testing.T, s string) {
	err := testparse(s)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
}

/*

	b := new(bytes.Buffer)
	s := new(Source)
	_, err := s.ParseLine("//wire9 S dstid[4] srcid[4] fontid[4] rpcmux[2] quid[8]\n")
	if err != nil {
		t.Errorf("unexpected error:", err)
		t.Fail()
	}
	if err := s.Generate(b); err != nil {
		t.Errorf("unexpected error:", err)
		t.Fail()
	}
	fmt.Fprintln(os.Stderr, b.Bytes())
*/
