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

// Corners.
const (
    TL = 0
    TR = 1
    BR = 2
    BL = 3
)

type point struct {
    x int
    y int
}

type sample []point

type Lcd struct {
    name string
    bb []point
    scaled []point
    line int
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
            if len(v) == 7 || len(v) == 9 {
                lcd := &Lcd{name:c.Keyword, bb:[]point{point{0,0}, point{v[0],v[1]}, point{v[2],v[3]}, point{v[4],v[5]}}, line:v[6]}
                // Initialise the sample lists
                if len(v) == 9 {
                    lcd.decimal = []point{{lcd.bb[BR].x + v[7], lcd.bb[BR].y + v[8]}}
                }
                // A line width is specified, so shrink the bounding box by 1/2 the line width
                lcd.scaled = shrink(lcd.bb, lcd.line/2)
                tl := lcd.scaled[TL]
                tr := lcd.scaled[TR]
                br := lcd.scaled[BR]
                bl := lcd.scaled[BL]
                // Middle points.
                mr := split(tr, br, 2)[0]
                ml := split(tl, bl, 2)[0]
                // For sampling the 'off' value, sample the middle of each of the 2 halves by
                // Taking 3 samples through the axis and dropping the middle one.
                lcd.off = split(split(tl, tr, 2)[0], split(bl, br, 2)[0], 4)
                lcd.off = sample{lcd.off[0], lcd.off[2]}
                lcd.segments = make([]sample, 7)
                // The assignments must match the bit allocation in
                // the lookup table.
                // Top left
                lcd.segments[0] = split(ml, tl, 3)
                // Top
                lcd.segments[1] = split(tl, tr, 3)
                // Top right
                lcd.segments[2] = split(tr, mr, 3)
                // Bottom right
                lcd.segments[3] = split(mr, br, 3)
                // Bottom
                lcd.segments[4] = split(br, bl, 3)
                // Bottom left
                lcd.segments[5] = split(bl, ml, 3)
                // Middle
                lcd.segments[6] = split(ml, mr, 3)
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

func (l *LcdDecoder) Decode(img image.Image) ([]string, []bool) {
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

// Return a slice of points that splits the line into sections.
func split(start, end point, sections int) []point {
    lx := end.x - start.x
    ly := end.y - start.y
    p := make([]point, sections - 1)
    for i := 1; i < sections; i++ {
        p[i-1] = point{start.x + lx * i / sections, start.y + ly * i / sections}
    }
    return p
}

// Shrink the bounding box by the selected size.
func shrink(bb []point, s int) ([]point) {
    if s == 0 {
        return bb
    }
    tl := bb[TL]
    br := bb[BR]
    nb := make([]point, 4)
    if tl.x < br.x && tl.y < br.y {
        nb[TL] = point{bb[TL].x + s, bb[TL].y + s}
        nb[TR] = point{bb[TR].x - s, bb[TR].y + s}
        nb[BR] = point{bb[BR].x - s, bb[BR].y - s}
        nb[BL] = point{bb[BL].x + s, bb[BL].y - s}
    } else if tl.x > br.x && tl.y < br.y {   // 90 degrees rotated clockwise
        nb[TL] = point{bb[TL].x - s, bb[TL].y + s}
        nb[TR] = point{bb[TR].x - s, bb[TR].y - s}
        nb[BR] = point{bb[BR].x + s, bb[BR].y - s}
        nb[BL] = point{bb[BL].x + s, bb[BL].y + s}
    } else if tl.x > br.x && tl.y > br.y {   // 180 degress rotated
        nb[TL] = point{bb[TL].x - s, bb[TL].y - s}
        nb[TR] = point{bb[TR].x + s, bb[TR].y - s}
        nb[BR] = point{bb[BR].x + s, bb[BR].y + s}
        nb[BL] = point{bb[BL].x - s, bb[BL].y + s}
    } else {        // 270 degress rotated
        nb[TL] = point{bb[TL].x + s, bb[TL].y - s}
        nb[TR] = point{bb[TR].x + s, bb[TR].y + s}
        nb[BR] = point{bb[BR].x - s, bb[BR].y + s}
        nb[BL] = point{bb[BL].x - s, bb[BL].y - s}
    }
    return nb
}

// Return an average of the sampled points.
func takeSample(img image.Image, d *Digit, slist sample) int {
    var g int
    for _, s := range slist {
        pix := color.Gray16Model.Convert(img.At(d.pos.x + s.x, d.pos.y + s.y)).(color.Gray16)
        g += int(pix.Y)
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
    white := color.RGBA{255, 255, 255, 255}
    for _, d := range l.digits {
        lcd := d.lcd
        drawCross(img, d, lcd.bb, white)
        drawCross(img, d, lcd.scaled, blue)
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
        for i := 1; i < 3; i++ {
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
