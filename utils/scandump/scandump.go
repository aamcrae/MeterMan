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
	"log"
	"os"
	"strconv"
)

var configFile = flag.String("config", "config", "Configuration file")
var section = flag.String("section", "meter", "Configuration section")
var input = flag.String("input", "input.jpg", "Input file")
var calibration = flag.String("calibration", "calibration", "Calibration cache file")

func init() {
	flag.Parse()
}

func main() {
	c, err := config.ParseFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to read config %s: %v", *configFile, err)
	}
	sect := c.GetSection(*section)
	var angle float64
	a, err := sect.GetArg("rotate")
	if err == nil {
		angle, err = strconv.ParseFloat(a, 64)
		if err != nil {
			angle = 0.0
		}
	}
	l, err := lcd.CreateLcdDecoder(sect)
	if err != nil {
		log.Fatalf("LCD config failed %v", err)
	}
	if f, err := os.Open(*calibration); err != nil {
		log.Fatalf("%s: %v\n", *calibration, err)
	} else {
		l.RestoreCalibration(f)
		f.Close()
	}
	inf, err := os.Open(*input)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", *input, err)
	}
	defer inf.Close()
	img, _, err := image.Decode(inf)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", *input, err)
	}
	if angle != 0 {
		img = lcd.RotateImage(img, angle)
	}
	fmt.Printf("Digit |  Min |  Off |  TL  |  TM  |  TR  |  BR  |  BM  |  BL  |  MM  |\n")
	for i, d := range l.Digits {
		min := d.Min()
		off := d.Off(img)
		max := d.Max()
		s := d.Samples(img)
		fmt.Printf("  %-2d  | %-5d| %-5d|", i, min, off)
		for _, v := range s {
			fmt.Printf(" %-5d|", v)
		}
		fmt.Printf("\n")
		fmt.Printf("         Max        |")
		for _, v := range max {
			fmt.Printf(" %-5d|", v)
		}
		fmt.Printf("\n")
		fmt.Printf("      |     0|  %-4d|", off - min)
		for i, v := range s {
			fmt.Printf(" %-5d|", perc(max[i], off, v))
		}
		fmt.Printf("\n")
	}
}

func perc(max, min, v int) int {
	return (v - min) * 100 / (max - min)
}
