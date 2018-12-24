package main

import (
    "fmt"
    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan"
    "image"
    "image/jpeg"
    "log"
    "os"
    "strings"
)

func main() {
    proc("i0000.jpg", "8.8.8.88.8.8.8.8.8.8.8")
}

func proc(imagename string, result string) {
    conf, err := config.ParseFile("real.config")
    if err != nil {
        log.Fatalf("Can't read config file: %v", err)
    }
    lcd, err := meterman.CreateLcdDecoder(conf)
    if err != nil {
        log.Fatalf("LCD config failed %v", err)
    }
    ifile, err := os.Open(imagename)
    if err != nil {
        log.Fatalf("%s: %v", imagename, err)
    }
    img, err := jpeg.Decode(ifile)
    if err != nil {
        log.Fatalf("Can't decode %s: %v", imagename, err)
    }
    // Convert image to gray scale.
    gi := image.NewGray(img.Bounds())
    b := img.Bounds()
    for y := b.Min.Y; y < b.Max.Y; y++ {
        for x := b.Min.X; x < b.Max.X; x++ {
            gi.Set(x, y, img.At(x, y))
        }
    }
    str, found := lcd.Decode(gi)
    got := strings.Join(str, "")
    if got != result {
        for i, f := range found {
            if !f {
                fmt.Printf("Element %d not found\n", i)
            }
        }
        log.Printf("For image %s, expected %s, found %s", imagename, result, got)
    }
}
