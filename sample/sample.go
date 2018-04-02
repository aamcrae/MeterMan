package main

import (
    "flag"
    "fmt"
    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan"
    "image"
    "image/color"
    "image/jpeg"
    "log"
    "os"
)

var output = flag.String("output", "output.jpg", "output jpeg file")
var configFile = flag.String("config", "", "Configuration file")
var input = flag.String("input", "", "Input jpeg file")

func init() {
    flag.Parse()
}

func main() {
    c, err := config.ParseFile(*configFile)
    if err != nil {
        log.Fatalf("Failed to read config %s: %v", *configFile, err)
    }
    lcd := meterman.NewLcdDecoder()
    err = lcd.Config(c)
    if err != nil {
        log.Fatalf("LCD config failed %v", err)
    }
    inf, err := os.Open(*input)
    if err != nil {
        log.Fatalf("Failed to open %s: %v", *input, err)
    }
    defer inf.Close()
    in, err := jpeg.Decode(inf)
    // Convert image to RGBA since jpeg images have no Set interface.
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
    lcd.MarkSamples(img)
    of, err := os.Create(*output)
    if err != nil {
        log.Fatalf("Failed to create %s: %v", *output, err)
    }
    defer of.Close()
    if err := jpeg.Encode(of, img, nil); err != nil {
        fmt.Printf("Error writing %s: %v\n", *output, err)
    } else {
        fmt.Printf("Wrote %s successfully\n", *output)
    }
}
