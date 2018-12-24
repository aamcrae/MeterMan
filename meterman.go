package meterman

import (
    "flag"
    "log"
    "strconv"

    "github.com/aamcrae/config"
)


var conf = flag.String("config", ".meterman", "Config file")

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
    decoder, err := CreateLcdDecoder(conf)
    if  err != nil {
        log.Printf("Failed to create decoder: %v", err);
    }
    for {
        img, err := GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        ni := ProcessImage(img, angle)
        chars, ok := decoder.Decode(ni)
        log.Printf("Len chars, ok = %d, %d", len(chars), len(ok))
    }
}
