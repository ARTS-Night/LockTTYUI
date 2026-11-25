package main

import (
 "strings"
 "testing"
 "time"
 "unicode/utf8"
)

func TestClockLines(t *testing.T) {
 now := time.Date(2026,11,25,20,59,26,0,time.UTC)
 for _, ascii := range []bool{false,true} {
  lines := clockLines(now,80,24,ascii)
  if len(lines)!=5 { t.Fatalf("expected five rows, got %d",len(lines)) }
  for _, line := range lines {
   if utf8.RuneCountInString(line)>80 { t.Fatal("clock overflows") }
   if ascii { for _, r := range line { if r>127 { t.Fatal("non-ASCII glyph") } } }
  }
  if strings.TrimSpace(lines[0])=="" { t.Fatal("empty clock") }
 }
 for _, size := range [][2]int{{38,24},{80,23}} {
  got := clockLines(now,size[0],size[1],false)
  if len(got)!=1 || got[0]!="20:59:26" { t.Fatal(got) }
 }
}

func TestAllClockDigits(t *testing.T) {
 for _, d := range "0123456789:" {
  glyph, ok := outlineDigits[d]
  if !ok { t.Fatalf("missing glyph %c",d) }
  width:=5
  if d==':' { width=1 }
  for _, row := range glyph { if utf8.RuneCountInString(row)!=width { t.Fatalf("bad width: %c %q",d,row) } }
 }
}
