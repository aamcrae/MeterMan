package meterman_test

import (
    "testing"

    "fmt"
    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan"
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
    digits, err := meterman.ConfigDigits(conf)
    if err != nil {
        t.Fatalf("Digit config failed %v", err)
    }
    fmt.Printf("Digits: %v\n", digits)
    ifile, err := os.Open(filepath.Join("testdata", imagename))
    if err != nil {
        t.Fatalf("%s: %v", imagename, err)
    }
    img, err := jpeg.Decode(ifile)
    if err != nil {
        t.Fatalf("Can't decode %s: %v", imagename, err)
    }
    _, _ = meterman.Decode(digits, img)
}
