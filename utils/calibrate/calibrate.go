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
	"image"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/lcd"
	"github.com/aamcrae/MeterMan/lib"
	"github.com/aamcrae/config"
)

var configFile = flag.String("config", "config", "Configuration file")
var read = flag.Bool("read", true, "If set, attempt to decode the digits.")
var train = flag.Bool("train", true, "Enable training mode")
var port = flag.Int("port", 8100, "Port for image server")
var refresh = flag.Int("refresh", 4, "Number of seconds before image refresh")
var delay = flag.Int("delay", 1, "Number of seconds between each image read")

func init() {
	flag.Parse()
}

func main() {
	server, err := serverInit(*port, *refresh)
	if err != nil {
		log.Fatalf("Server init failed %v", err)
	}
	client := http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	var fileMod time.Time
	var angle float64
	var source string
	var decoder *lcd.LcdDecoder
	for {
		var in image.Image
		// Check whether config file has changed.
		fi, err := os.Stat(*configFile)
		if err != nil {
			log.Fatalf("%s: %v", *configFile, err)
		}
		if fileMod != fi.ModTime() {
			time.Sleep(1 * time.Second)
			fileMod = fi.ModTime()
			c, err := config.ParseFile(*configFile)
			if err != nil {
				log.Fatalf("Failed to read config %s: %v", *configFile, err)
			}
			sect := c.GetSection("meter")
			// If training, save any existing calibration.
			if *train && decoder != nil {
			}
			a, err := sect.GetArg("rotate")
			if err == nil {
				angle, err = strconv.ParseFloat(a, 64)
				if err != nil {
					angle = 0.0
				}
			}
			source, _ = sect.GetArg("source")
			decoder, err = lcd.CreateLcdDecoder(sect)
			if err != nil {
				log.Fatalf("LCD config failed %v", err)
			}
			if *read || *train {
				cf, _ := sect.GetArg("calibration")
				if len(cf) != 0 {
					if _, err := decoder.RestoreFromFile(cf); err != nil {
						log.Printf("%s: %v\n", cf, err)
					}
				}
			}
			server.updateDecoder(decoder)
			log.Printf("Config file %s updated", *configFile)
		}
		res, err := client.Get(source)
		if err != nil {
			log.Fatalf("Failed to retrieve source image from %s: %v", source, err)
		}
		in, _, err = image.Decode(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatalf("Failed to decode image from %s: %v", source, err)
		}
		if angle != 0 {
			in = lib.RotateImage(in, angle)
		}
		var str strings.Builder
		if *read && decoder != nil {
			digits := decoder.Decode(in)
			for i := range digits.Digits {
				d := &digits.Digits[i]
				if d.Valid {
					str.WriteString(d.Str)
				} else {
					str.WriteRune('X')
				}
			}
			log.Printf("Segments = <%s>\n", str.String())
		}
		server.updateImage(in, str.String())
		if *delay >= 0 {
			time.Sleep(time.Duration(*delay) * time.Second)
		}
	}
}
