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

func main() {
    cam, err := meterman.OpenCamera(*device)
    if err != nil {
        log.Fatalf("%s: %v", *device, err)
    }
	defer cam.Close()
    if err := cam.Init(meterman.YUYV, *resolution); err != nil {
		log.Fatalf("Init failed: %v", err)
    }
    i := 0
    for  {
        frame, err := cam.GetFrame()
        if err != nil {
            log.Fatalf("Getframe: %v", err)
        }
        img := cam.ConvertRGBA(frame)
        fname := fmt.Sprintf("i%04d.jpg", i)
        of, err := os.Create(fname)
        if err != nil {
		     log.Fatalf("Failed to create %s: %v", fname, err)
        }
        if err := jpeg.Encode(of, img, nil); err != nil {
            fmt.Printf("Error writing %s: %v\n", fname, err)
        } else {
            fmt.Printf("Wrote %s successfull\n", fname)
        }
        of.Close()
        time.Sleep(time.Second)
        i++
	}
}
