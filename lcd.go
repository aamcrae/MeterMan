package meterman

import (
    "github.com/aamcrae/config"
    "fmt"
    "image"
    "strconv"
)

type Digit struct {
    values [8]int
    decimalX int
    decimalY int
}

// There are 128 possible values in a 7 segment display,
// and this table maps each one to a unique string.
// The segments are used as a bit in 7 bit value, and mapped as:
// top left     bit 0
// top          bit 1
// top right    bit 2
// bottom right bit 3
// bottom       bit 4
// bottom left  bit 5
// middle       bit 6
const (
  X = 0
  s_tl = 1
  s_t = 2
  s_tr = 4
  s_br = 8
  s_b = 0x10
  s_bl = 0x20
  s_m = 0x40
)

var resultTable = map[int]string {
    s_tl | s_t  | s_tr | s_br | s_b  | s_bl |  X   : "0",
     X   |  X   | s_tr | s_br |  X   |  X   |  X   : "1",
     X   | s_t  | s_tr |  X   | s_b  | s_bl | s_m  : "2",
     X   | s_t  | s_tr | s_br | s_b  |  X   | s_m  : "3",
    s_tl |  X   | s_tr | s_br |  X   |  X   | s_m  : "4",
    s_tl | s_t  |  X   | s_br | s_b  |  X   | s_m  : "5",
    s_tl | s_t  |  X   | s_br | s_b  | s_bl | s_m  : "6",
    s_tl | s_t  | s_tr | s_br |  X   |  X   |  X   : "7",
    s_tl | s_t  | s_tr | s_br | s_b  | s_bl | s_m  : "8",
    s_tl | s_t  | s_tr | s_br | s_b  |  X   | s_m  : "9",
}

func ConfigDigits(conf config.Config) ([]Digit, error) {
    digits := []Digit{}
    var decimalX, decimalY int
    tok, ok := conf.GetTokens("decimal")
    if ok {
        if len(tok) != 2 {
            return nil, fmt.Errorf("Bad config for 'decimal'")
        }
        x, err := strconv.ParseInt(tok[0], 10, 32)
        if err != nil {
            return nil, fmt.Errorf("Bad X value for 'decimal'")
        }
        y, err := strconv.ParseInt(tok[1], 10, 32)
        if err != nil {
            return nil, fmt.Errorf("Bad Y value for 'decimal'")
        }
        decimalX = int(x)
        decimalY = int(y)
    } else {
        fmt.Printf("Warning - no decimal offset\n")
    }
    for i := 1; true; i++{
        digit := Digit{}
        digName := fmt.Sprintf("digit%d", i)
        tok, ok := conf.GetTokens(digName)
        if !ok {
            break
        }
        if len(tok) != 8 {
            return nil, fmt.Errorf("Bad config for %s", digName)
        }
        for j, t := range tok {
            if v, err := strconv.ParseInt(t, 10, 32); err != nil {
                return nil, fmt.Errorf("Bad config for %s", digName)
            } else {
                digit.values[j] = int(v)
            }
        }
        digit.decimalX = int(decimalX)
        digit.decimalY = int(decimalY)
        digits = append(digits, digit)
        // Validate ranges.
    }
    return digits, nil
}

func Decode(digits []Digit, img image.Image) ([]string, []bool) {
    return []string{}, []bool{}
}
