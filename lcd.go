package meterman

import (
    "github.com/aamcrae/config"
    "fmt"
    "image"
    "image/color"
    "strconv"
    "strings"
)

// Default threshold
const defaultThreshold = 20

type point struct {
    x int
    y int
}

type sample []point

type Lcd struct {
    name string
    w, h, offset, line int 
    off sample
    decimal sample
    segments []sample
}

type Digit struct {
    name string
    lcd *Lcd
    pos point
}

type LcdDecoder struct {
    digits []*Digit
    lcdMap map[string]*Lcd
    threshold int
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
   return &LcdDecoder{[]*Digit{}, map[string]*Lcd{}, defaultThreshold}
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
                // Initialise the sample lists
                cl := lcd.line/2
                w := lcd.w
                h := lcd.h
                o := lcd.offset
                if len(v) == 6 {
                    lcd.decimal = makeSamples(w + v[4], h + v[5], w + v[4], h + v[5], 1)
                }
                // Make 3 samples, and drop the middle one.
                lcd.off = makeSamples(w/2, 0, w/2 + o, h, 3)
                lcd.off = sample{lcd.off[0], lcd.off[2]}
                lcd.segments = make([]sample, 7)
                // The assignments must match the bit allocation in
                // the lookup table.
                // Top left
                lcd.segments[0] = makeSamples(cl, 0, cl + o/2, h/2, 2)
                // Top
                lcd.segments[1] = makeSamples(0, cl, w, cl, 2)
                // Top right
                lcd.segments[2] = makeSamples(w - cl, 0, w + o/2 - cl, h/2, 2)
                // Bottom right
                lcd.segments[3] = makeSamples(w + o/2 - cl, h/2, w + o - cl, h, 2)
                // Bottom
                lcd.segments[4] = makeSamples(o, h - cl, w + o - cl, h - cl, 2)
                // Bottom left
                lcd.segments[5] = makeSamples(o/2 + cl, h/2, o + cl, h, 2)
                // Middle
                lcd.segments[6] = makeSamples(o/2, h/2, w + o/2, h/2, 2)
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
    t, _ := conf.GetTokens("threshold")
    v := readInts(t)
    if len(v) == 1 {
        l.threshold = v[0]
    }
    return nil
}

func (l *LcdDecoder) Decode(img *image.Gray) ([]string, []bool) {
    strs := []string{}
    ok := []bool{}
    for _, d := range l.digits {
        lcd := d.lcd
        // Find off point.
        off := takeSample(img, d, lcd.off)
        // Considered on if darker by a set threshold.
        on := off - ((off * l.threshold) / 100)
        var decimal bool = false
        if len(lcd.decimal) != 0 && on >= takeSample(img, d, lcd.decimal) {
            decimal = true
        }
        lookup := 0
        for seg, s := range lcd.segments {
            pixel := takeSample(img, d, s)
            if on >= pixel {
                lookup |= 1 << uint(seg)
            }
        }
        result, found := resultTable[lookup]
        if decimal {
            result = result + "."
        }
        strs = append(strs, result)
        ok = append(ok, found)
    }
    return strs, ok
}

// Make a slice of samples between the two points.
func makeSamples(sx, sy, ex, ey, count int) []point {
    lx := ex - sx
    ly := ey - sy
    div := count + 1
    p := make([]point, count)
    for i := 1; i <= count; i++ {
        p[i-1] = point{sx + lx * i / div, sy + ly * i / div}
    }
    return p
}

// Return an average of the sampled points.
func takeSample(img *image.Gray, d *Digit, slist sample) int {
    var g int
    for _, s := range slist {
        g += int(img.GrayAt(d.pos.x + s.x, d.pos.y + s.y).Y)
    }
    return g / len(slist)
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

// Mark the samples with a red cross.
func (l *LcdDecoder) MarkSamples(img *image.RGBA) {
    red := color.RGBA{255, 0, 0, 255}
    green := color.RGBA{0, 255, 0, 255}
    blue := color.RGBA{0, 0, 255, 255}
    basepoint := []point{point{0, 0}}
    for _, d := range l.digits {
        lcd := d.lcd
        drawCross(img, d, basepoint, blue)
        drawCross(img, d, lcd.off, green)
        if len(lcd.decimal) != 0 {
            drawCross(img, d, lcd.decimal, red)
        }
        for _, s := range lcd.segments {
            drawCross(img, d, s, red)
        }
    }
}

func drawCross(img *image.RGBA, d *Digit, s sample, c color.Color) {
    for _, p := range s {
        x := d.pos.x + p.x
        y := d.pos.y + p.y
        img.Set(x, y, c)
        for i := 1; i < 4; i++ {
            img.Set(x - i, y, c)
            img.Set(x + i, y, c)
            img.Set(x, y - i, c)
            img.Set(x, y + i, c)
        }
    }
}

func printSamples(s []point) {
    for _, p := range s {
        fmt.Printf("x = %d, y = %d\n", p.x, p.y)
    }
}
