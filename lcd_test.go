package meterman_test

import (
    "testing"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan"
    "image"
    "image/jpeg"
    "os"
    "path/filepath"
)

func TestImg1(t *testing.T) {
    runTest(t, "test1", "12345678")
}

func runTest(t *testing.T, name string, result string) {
    cname := name + ".config"
    imagename := name + ".jpg"
    conf, err := config.ParseFile(filepath.Join("testdata", cname))
    if err != nil {
        t.Fatalf("Can't read config %s: %v", cname, err)
    }
    lcd := meterman.NewLcdDecoder()
    err = lcd.Config(conf)
    if err != nil {
        t.Fatalf("LCD config failed %v", err)
    }
    ifile, err := os.Open(filepath.Join("testdata", imagename))
    if err != nil {
        t.Fatalf("%s: %v", imagename, err)
    }
    img, err := jpeg.Decode(ifile)
    if err != nil {
        t.Fatalf("Can't decode %s: %v", imagename, err)
    }
    // Convert image to gray scale.
    gi := image.NewGray(img.Bounds())
    b := img.Bounds()
    for y := b.Min.Y; y < b.Max.Y; y++ {
        for x := b.Min.X; x < b.Max.X; x++ {
            gi.Set(x, y, img.At(x, y))
        }
    }
    exp := []string{"1", "2", "3", "4", "5", "6", "7", "8."}
    str, found := lcd.Decode(gi)
    for i, s := range str {
        if !found[i] {
            t.Fatalf("Digit %d, result not found", i)
        }
        if s != exp[i] {
            t.Fatalf("Digit %d, exp '%s', got %s", i, exp[i], s)
        }
    }
}
