// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"github.com/aamcrae/MeterMan/lcd"
	"github.com/aamcrae/config"
	"image"
	"image/color"
	"log"
	"os"
	"strconv"
)

var output = flag.String("output", "output.jpg", "output jpeg file")
var configFile = flag.String("config", "config", "Configuration file")
var input = flag.String("input", "input.png", "Input file")
var process = flag.Bool("process", true, "Decode digits in image")

func init() {
	flag.Parse()
}

func main() {
	c, err := config.ParseFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to read config %s: %v", *configFile, err)
	}
	sect := c.GetSection("meter")
	var angle float64
	a, err := sect.GetArg("rotate")
	if err == nil {
		angle, err = strconv.ParseFloat(a, 64)
		if err != nil {
			angle = 0.0
		}
	}
	l, err := lcd.CreateLcdDecoder(sect)
	s := sect.Get("calibrate")
	if len(s) == 1 && len(s[0].Tokens) == 1 {
		img, err := lcd.ReadImage(s[0].Tokens[0])
		if err != nil {
			log.Fatalf("%v", err)
		}
		l.Calibrate(img)
	}
	if err != nil {
		log.Fatalf("LCD config failed %v", err)
	}
	inf, err := os.Open(*input)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", *input, err)
	}
	defer inf.Close()
	in, _, err := image.Decode(inf)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", *input, err)
	}
	if angle != 0 {
		in = lcd.RotateImage(in, angle)
	}
	// Convert image to RGBA.
	b := in.Bounds()
	img := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.Set(x, y, color.RGBAModel.Convert(in.At(x, y)))
		}
	}
	if *process {
		vals, ok := l.Decode(img)
		for i, v := range vals {
			fmt.Printf("segment %d = '%s', ok = %v\n", i, v, ok[i])
		}
	}
	l.MarkSamples(img)
	err = lcd.SaveImage(*output, img)
	if err != nil {
		log.Fatalf("%s encode error: %v", *output, err)
	}
	fmt.Printf("Wrote %s successfully\n", *output)
}
