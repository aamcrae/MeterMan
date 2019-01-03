package main

import (
    "flag"
    "fmt"
    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/lcd"
    "image"
    "image/color"
    "log"
    "os"
)

var output = flag.String("output", "output.jpg", "output jpeg file")
var configFile = flag.String("config", "config", "Configuration file")
var input = flag.String("input", "input.png", "Input file")

func init() {
    flag.Parse()
}

func main() {
    c, err := config.ParseFile(*configFile)
    if err != nil {
        log.Fatalf("Failed to read config %s: %v", *configFile, err)
    }
    l, err := lcd.CreateLcdDecoder(c)
    s := c.Get("calibrate")
    if len(s) == 1 && len(s[0].Tokens) == 1 {
        img, err := lcd.ReadImage(s[0].Tokens[0])
        if  err != nil {
            log.Fatalf("%v", err);
        }
        l.Calibrate(img)
    }
    if err != nil {
        log.Fatalf("LCD config failed %v", err)
    }
    inf, err := os.Open(*input)
    if err != nil {
        log.Fatalf("Failed to open %s: %v", *input, err)
    }
    defer inf.Close()
    in, _, err := image.Decode(inf)
    // Convert image to RGBA.
    b := in.Bounds()
    img := image.NewRGBA(b)
    for y := b.Min.Y; y < b.Max.Y; y++ {
        for x := b.Min.X; x < b.Max.X; x++ {
            img.Set(x, y, color.RGBAModel.Convert(in.At(x, y)))
        }
    }
    if err != nil {
        log.Fatalf("Failed to read %s: %v", *input, err)
    }
    vals, ok := l.Decode(img)
    for i, v := range vals {
        fmt.Printf("segment %d = '%s', ok = %v\n", i, v, ok[i])
    }
    l.MarkSamples(img)
    err = lcd.SaveImage(*output, img)
    if err != nil {
        log.Fatalf("%s encode error: %v", *output, err)
    }
    fmt.Printf("Wrote %s successfully\n", *output)
}