package meterman

import (
    "image"

    "github.com/fogleman/gg"
)

// Rotate the image, using a max sized canvas.
func ProcessImage(image image.Image, angle float64, size int) image.Image {
    // Create a canvas of the selected size.
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
