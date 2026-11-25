package main

import (
 "strings"
 "time"
)

var outlineDigits = map[rune][5]string{
 '0': {"┌───┐","│   │","│   │","│   │","└───┘"},
 '1': {"  ┌─┐","    │","    │","    │","    │"},
 '2': {"┌───┐","    │","┌───┘","│    ","└────"},
 '3': {"────┐","    │"," ───┤","    │","────┘"},
 '4': {"│   │","│   │","└───┤","    │","    │"},
 '5': {"┌────","│    ","└───┐","    │","────┘"},
 '6': {"┌────","│    ","├───┐","│   │","└───┘"},
 '7': {"────┐","    │","    │","    │","    │"},
 '8': {"┌───┐","│   │","├───┤","│   │","└───┘"},
 '9': {"┌───┐","│   │","└───┤","    │","────┘"},
 ':': {" ","o"," ","o"," "},
}

func clockLines(t time.Time, width, height int, ascii bool) []string {
 stamp := t.Format("15:04:05")
 // Six five-cell digits, two one-cell separators and seven spaces.
 if width < 39 || height < 24 {
  return []string{stamp}
 }
 lines := make([]string, 5)
 for _, digit := range stamp {
  glyph := outlineDigits[digit]
  for i, s := range glyph {
   if lines[i] != "" { lines[i] += " " }
   lines[i] += s
  }
 }
 replacer := strings.NewReplacer("┌","+","┐","+","└","+","┘","+","├","+","┤","+","─","-","│","|")
 for i, line := range lines {
  if ascii { line = replacer.Replace(line) }
  n := len([]rune(line))
  lines[i] = strings.Repeat(" ", max(0,(width-n)/2)) + line
 }
 return lines
}
