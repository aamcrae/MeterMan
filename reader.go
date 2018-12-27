package main

import (
    "image"
    "net/http"
    "math"

    "github.com/fogleman/gg"
)


// Rotate the image, using a max sized canvas.
func ProcessImage(image image.Image, angle float64) image.Image {
    // Create a canvas of the maximum size required.
    dx := float64(image.Bounds().Dx())
    dy := float64(image.Bounds().Dy())
    size := int(math.Sqrt(dx * dx + dy * dy) + 1.0)
    c := gg.NewContext(size, size)
    width := image.Bounds().Dx()
    height := image.Bounds().Dy()
    startx := (size - width)/2
    starty := (size - height)/2
    if angle != 0 {
        c.RotateAbout(gg.Radians(angle), float64(size/2), float64(size/2))
    }
    c.DrawImage(image, startx, starty)
    return c.Image()
}

func GetSource(src string) (image.Image, error) {
    res, err := http.Get(src)
	if err != nil {
		return nil, err
	}
    img, _, err := image.Decode(res.Body)
	res.Body.Close()
    return img, err
}
