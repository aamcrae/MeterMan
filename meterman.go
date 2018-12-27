package main

import (
    "flag"
    "image"
    "image/jpeg"
    "log"
    "os"
    "strconv"
    "strings"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/lcd"
)


var conf = flag.String("config", ".meterman", "Config file")
var verbose = flag.Bool("verbose", false, "Verbose tracing")
var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")

func init() {
    flag.Parse()
}

func main() {
    conf, err := config.ParseFile(*conf)
    if err != nil {
        log.Fatalf("Can't read config %s: %v", *conf, err)
    }
    var angle float64
    a := conf.Get("rotate")
    if len(a) == 1 {
        if len(a[0].Tokens) != 1 {
            log.Fatalf("Bad rotate configuration at %s:%d", a[0].Filename, a[0].Lineno)
        }
        angle, err = strconv.ParseFloat(a[0].Tokens[0], 64)
        if err != nil {
            log.Fatalf("Bad rotate parameter at %s:%d", a[0].Filename, a[0].Lineno)
        }
    }
    s := conf.Get("source")
    if len(s) != 1 {
        log.Fatalf("Missing or bad 'source' configuration")
    }
    if len(s[0].Tokens) != 1 {
        log.Fatalf("Bad source configuration at %s:%d", s[0].Filename, s[0].Lineno)
    }
    source := s[0].Tokens[0]
    decoder, err := lcd.CreateLcdDecoder(conf)
    if  err != nil {
        log.Printf("Failed to create decoder: %v", err);
    }
    for {
        img, err := GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        if *verbose {
            log.Printf("Read source img: %d x %d", img.Bounds().Max.X, img.Bounds().Max.Y)
        }
        ni := ProcessImage(img, angle)
        vals, ok := decoder.Decode(ni)
        for _, okDigit := range ok {
            if !okDigit {
                badRead(ni, vals, ok)
            }
        }
        key := strings.Join(vals[0:4], "")
        value := strings.Join(vals[4:], "")
        log.Printf("Key = %s, value = %s", key, value)
    }
}

func badRead(img image.Image, vals []string, ok []bool) {
    log.Printf("Bad read:\n")
    for i, v := range vals {
       log.Printf("segment %d = '%s', ok = %v\n", i, v, ok[i])
    }
    if *saveBad {
        saveImage(*badFile, img)
    }
}

func saveImage(name string, img image.Image) {
    of, err := os.Create(name)
    if err != nil {
        log.Printf("Failed to create image file %s: %v", name, err)
        return
    }
    defer of.Close()
    if err := jpeg.Encode(of, img, nil); err != nil {
        log.Printf("Error writing display file %s: %v\n", name, err)
        return
    }
}
