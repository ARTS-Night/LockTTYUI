package main

import "testing"

func TestCPU(t *testing.T) {
	if v := cpuUsage(cpu{100, 60}, cpu{200, 120}); v != 40 {
		t.Fatal(v)
	}
	if cpuUsage(cpu{100, 60}, cpu{100, 60}) != -1 {
		t.Fatal("invalid delta")
	}
}
func TestMemory(t *testing.T) {
	v, e := memory()
	if e != nil || v < 0 || v > 100 {
		t.Fatal(v, e)
	}
}
func TestHistory(t *testing.T) {
	h := push([]float64{1, 2, 3}, 4)
	if len(h) != 3 || h[0] != 2 || h[2] != 4 {
		t.Fatal(h)
	}
}
func TestBarRows(t *testing.T) {
 rows := barRows([]float64{0,100},2,2,100,false,true,"6")
 if len(rows)!=2 {t.Fatal(rows)}
 for _,row:=range rows {if visibleWidth(row)!=2 {t.Fatal(row)}}
 if rows[0]!=" \x1b[46m \x1b[0m" {t.Fatalf("unexpected fill: %q",rows[0])}
}
