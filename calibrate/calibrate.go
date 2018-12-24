package main

import (
    "flag"
    "image"
    "image/gif"
    "image/jpeg"
    "image/png"
    "log"
    "math"
    "os"
    "strings"

    "github.com/aamcrae/MeterMan"
)

var angle = flag.Float64("angle", 216.2, "Rotation angle (degrees clockwise)")
var input = flag.String("input", "input.png", "Input image (png, jpg, gif")
var output = flag.String("output", "output.png", "Output image (png, jpg, gif")

func init() {
    flag.Parse()
}

func main() {
    ifile, err := os.Open(*input)
    if err != nil {
        log.Fatalf("%s: %v", *input, err)
    }
    defer ifile.Close()
    img, _, err := image.Decode(ifile)
    if err != nil {
        log.Fatalf("%s: %v", *input, err)
    }
    dx := float64(img.Bounds().Dx())
    dy := float64(img.Bounds().Dy())
    maxsize := int(math.Sqrt(dx * dx + dy * dy) + 1.0)
    result := meterman.ProcessImage(img, *angle, maxsize)
    of, err := os.Create(*output)
    if err != nil {
        log.Fatalf("%s: %v", *output, err)
    }
    defer of.Close()
    if strings.HasSuffix(*output, "png") {
        err = png.Encode(of, result)
    } else if strings.HasSuffix(*output, "jpg") {
        err = jpeg.Encode(of, result, nil)
    } else if strings.HasSuffix(*output, "gif") {
        err = gif.Encode(of, result, nil)
    } else {
        log.Fatalf("%s: unknown image format", *output)
    }
    if err != nil {
        log.Fatalf("%s encode error: %v", *output, err)
    }
}
