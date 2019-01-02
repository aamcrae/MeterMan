package main

import (
    "flag"
    "fmt"
    "github.com/aamcrae/MeterMan"
    "image/jpeg"
    "log"
    "os"
    "time"
)

var device = flag.String("input", "/dev/video0", "Input video device")
var resolution = flag.String("resolution", "800x600", "Selected resolution of camera")
var format = flag.String("format", "YUYV 4:2:2", "Selected pixel format of camera")
var query = flag.Bool("query", false, "Display pixel formats and resolution")
var delay = flag.Float64("delay", 1.0, "Delay in seconds between grabs")

func init() {
    flag.Parse()
}

func main() {
    cam, err := meterman.OpenCamera(*device)
    if err != nil {
        log.Fatalf("%s: %v", *device, err)
    }
	defer cam.Close()
    if *query {
        m := cam.Query()
        fmt.Printf("%s:\n", *device)
        for k, v := range m {
            fmt.Printf("    %s:\n", k)
            for _, r := range v {
                fmt.Printf("        %s:\n", r)
            }
        }
        return
    }
    if err := cam.Init(*format, *resolution); err != nil {
		log.Fatalf("Init failed: %v", err)
    }
    i := 0
    sleep := time.Duration(int64(float64(time.Second) * *delay))
    for  {
        frame, err := cam.GetFrame()
        if err != nil {
            log.Fatalf("Getframe: %v", err)
        }
        fname := fmt.Sprintf("i%04d.jpg", i)
        of, err := os.Create(fname)
        if err != nil {
		     log.Fatalf("Failed to create %s: %v", fname, err)
        }
        if err := jpeg.Encode(of, frame, nil); err != nil {
            fmt.Printf("Error writing %s: %v\n", fname, err)
        } else {
            fmt.Printf("Wrote %s successfully\n", fname)
        }
        frame.Release()
        of.Close()
        time.Sleep(sleep)
        i++
	}
}
