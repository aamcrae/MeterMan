package meterman

import (
    "github.com/aamcrae/config"
    "fmt"
    "image"
    "strconv"
    "strings"
)

type point struct {
    x int
    y int
}

type Lcd struct {
    name string
    w, h, offset, line, decimalX, decimalY int
    off []point
}

type Digit struct {
    name string
    descriptor *Lcd
    pos point
}

type LcdDecoder struct {
    digits []*Digit
    lcdMap map[string]*Lcd
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
     X   |  X   |  X   |  X   |  X   |  X   |  X   : " ",
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

func NewLcdDecoder() *LcdDecoder {
   return &LcdDecoder{[]*Digit{}, map[string]*Lcd{}}
}

func (l *LcdDecoder) Config(conf *config.Config) error {
    for _, c := range conf.Entries {
        if strings.HasPrefix(c.Keyword, "lcd") {
            if _, ok := l.lcdMap[c.Keyword]; ok {
                return fmt.Errorf("Duplicate LCD entry: %s", c.Keyword)
            }
            v := readInts(c.Tokens)
            if len(v) == 4 || len(v) == 6 {
                lcd := &Lcd{name:c.Keyword, w:v[0], h:v[1], offset:v[2], line:v[3]}
                if len(v) == 6 {
                    lcd.decimalX = v[4]
                    lcd.decimalY = v[5]
                }
                for i := 1; i < 3; i++ {
                    fmt.Printf("off = %d %d\n", lcd.w/2, (lcd.h / 3) * i)
                    lcd.off = append(lcd.off, point{lcd.w/2, (lcd.h / 3) * i})
                }
                l.lcdMap[c.Keyword] = lcd
            } else {
                return fmt.Errorf("Bad config for LCD '%s'", c.Keyword)
            }
        } else if strings.HasPrefix(c.Keyword, "digit") {
            if len(c.Tokens) != 3 {
                return fmt.Errorf("Bad digit config for %s", c.Keyword)
            }
            lcd, ok := l.lcdMap[c.Tokens[0]]
            if !ok {
                return fmt.Errorf("Missing LCD %s for digit %s", c.Tokens[0], c.Keyword)
            }
            v := readInts(c.Tokens[1:])
            if len(v) != 2 {
                return fmt.Errorf("Bad config for digit %s", c.Keyword)
            }
            l.digits = append(l.digits, &Digit{c.Keyword, lcd, point{v[0], v[1]}})
        }
    }
    return nil
}

func (l *LcdDecoder) Decode(img *image.Gray) ([]string, []bool) {
    strs := []string{}
    ok := []bool{}
    for i, d := range l.digits {
        lcd := d.descriptor
        var off int
        // Find off point.
        for _, o := range lcd.off {
            g := int(img.GrayAt(d.pos.x + o.x, d.pos.y + o.y).Y)
            off += g
            fmt.Printf("digit %d: x = %d, y = %d, val=%d\n", i, d.pos.x + o.x, d.pos.y + o.y, g)
        }
        off = off / len(lcd.off)
        fmt.Printf("off avg for digit %d = %d\n", i, off)
    }
    return strs, ok
}

func readInts(strs []string) []int {
    vals := []int{}
    for _, s := range strs {
        if v, err := strconv.ParseInt(s, 10, 32); err != nil {
            break
        } else {
            vals = append(vals, int(v))
        }
    }
    return vals
}
