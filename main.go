package main

import "flag"
import "fmt"
import "image/jpeg"
import "log"
import "os"
import "time"

var device = flag.String("input", "/dev/video0", "Input video device")

func main() {
    cam, err := OpenCamera(*device)
    if err != nil {
        log.Fatalf("%s: %v", *device, err)
    }
	defer cam.Close()
    if err := cam.Init(YUYV, "800x600"); err != nil {
		log.Fatalf("Init failed: %v", err)
    }
    for i := 0; i < 5; i++ {
        img, err := cam.GetFrame()
        if err != nil {
            log.Fatalf("Getframe: %v", err)
        }
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
	}
}
