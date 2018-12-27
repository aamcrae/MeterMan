package main

import (
    "flag"
    "fmt"
    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/lcd"
    "image"
    "image/color"
    "image/gif"
    "image/jpeg"
    "image/png"
    "log"
    "os"
    "strings"
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
    lcd, err := lcd.CreateLcdDecoder(c)
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
    vals, ok := lcd.Decode(img)
    for i, v := range vals {
        fmt.Printf("segment %d = '%s', ok = %v\n", i, v, ok[i])
    }
    lcd.MarkSamples(img)
    of, err := os.Create(*output)
    if err != nil {
        log.Fatalf("Failed to create %s: %v", *output, err)
    }
    defer of.Close()
    if strings.HasSuffix(*output, "png") {
        err = png.Encode(of, img)
    } else if strings.HasSuffix(*output, "jpg") {
        err = jpeg.Encode(of, img, nil)
    } else if strings.HasSuffix(*output, "gif") {
        err = gif.Encode(of, img, nil)
    } else {
        log.Fatalf("%s: unknown image format", *output)
    }
    if err != nil {
        log.Fatalf("%s encode error: %v", *output, err)
    }
    fmt.Printf("Wrote %s successfully\n", *output)
}
