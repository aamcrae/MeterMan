package main

import (
    "flag"
    "log"
    "strconv"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/reader"
)


var conf = flag.String("config", ".meterman", "Config file")
var verbose = flag.Bool("verbose", false, "Verbose tracing")
var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")

type Result struct {
    tag string
    value float64
}

var WritersInit []func (*config.Config) (chan <- Result, error)

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
    source, err := conf.GetArg("source")
    if err != nil {
        log.Fatalf("%v", err)
    }
    r, err := reader.NewReader(conf)
    if  err != nil {
        log.Printf("Failed to create reader: %v", err);
    }
    s := conf.Get("calibrate")
    if len(s) == 1 && len(s[0].Tokens) == 1 {
        img, err := reader.ReadImage(s[0].Tokens[0])
        if  err != nil {
            log.Fatalf("%v", err);
        }
        r.Calibrate(img)
    }
    var wr []chan<-Result
    for _, wi := range WritersInit {
        if c, err := wi(conf); err != nil {
            log.Fatalf("Writer init failed: %v", err)
        } else {
            wr = append(wr, c)
        }
    }
    for {
        img, err := reader.GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        img = reader.RotateImage(img, angle)
        tag, val, err := r.Read(img)
        if err != nil {
            log.Printf("Read error: %v", err)
            if *saveBad {
                reader.SaveImage(*badFile, img)
            }
        } else if len(tag) > 0 {
            if *verbose {
                log.Printf("Tag: %s value %f\n", tag, val)
            }
            res := Result{tag, val}
            for _, c := range wr {
                c<- res
            }
        }
    }
}
