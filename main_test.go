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
func TestPlot(t *testing.T) {
	g := plot([]float64{0, 100, 100, 0}, 4, 3, false)
	if string(g[0]) != " ┌─┐" || string(g[2]) != "─┘ └" {
		t.Fatalf("%q / %q", string(g[0]), string(g[2]))
	}
	a := plot([]float64{-1, 0, 100}, 3, 3, true)
	for _, row := range a {
		for _, r := range row {
			if r > 127 {
				t.Fatal(r)
			}
		}
	}
}
