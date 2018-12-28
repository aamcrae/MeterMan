package main

import (
    "flag"
    "image"
    "log"
    "os"

    "github.com/aamcrae/MeterMan/reader"
)

var angle = flag.Float64("angle", 215.5, "Rotation angle (degrees clockwise)")
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
    result := reader.RotateImage(img, *angle)
    err = reader.SaveImage(*output, result)
    if err != nil {
        log.Fatalf("%s: %v", *output, err)
    }
}
