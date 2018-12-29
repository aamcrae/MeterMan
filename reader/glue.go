package reader

import (
    "flag"
    "log"
    "strconv"
    "time"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)


var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")
var sampleTime = flag.Int("sample", 2, "Sample time (seconds)")


func init() {
    core.RegisterReader(lcdReader)
}

func lcdReader(conf *config.Config, wr []chan<- core.Result) error {
    var angle float64
    a, err := conf.GetArg("rotate")
    if err != nil {
        return err
    }
    angle, err = strconv.ParseFloat(a, 64)
    if err != nil {
        return err
    }
    source, err := conf.GetArg("source")
    if err != nil {
        return err
    }
    r, err := NewReader(conf)
    if  err != nil {
        return err
    }
    s := conf.Get("calibrate")
    if len(s) == 1 && len(s[0].Tokens) == 1 {
        img, err := ReadImage(s[0].Tokens[0])
        if  err != nil {
            return err
        }
        r.Calibrate(img)
    }
    go runReader(r, source, angle, wr)
    return nil
}

func runReader(r *Reader, source string, angle float64, wr []chan<- core.Result) {
    delay := time.Duration(*sampleTime) * time.Second
    lastTime := time.Now()
    for {
        img, err := GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        img = RotateImage(img, angle)
        tag, val, err := r.Read(img)
        if err != nil {
            log.Printf("Read error: %v", err)
            if *saveBad {
                SaveImage(*badFile, img)
            }
        } else if len(tag) > 0 {
            if *core.Verbose {
                log.Printf("Tag: %s value %f\n", tag, val)
            }
            res := core.Result{tag, val}
            for _, c := range wr {
                c<- res
            }
        }
        time.Sleep(delay - time.Now().Sub(lastTime))
        lastTime = time.Now()
    }
}
