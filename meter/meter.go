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

package meter

import (
	"flag"
	"image"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lcd"
)

var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")
var sampleTime = flag.Int("sample", 4900, "Image sample rate (milliseconds)")
var sourceTimeout = flag.Int("source_timeout", 20, "Source timeout in seconds")

// Maps meter label to tag.
var tagMap map[string]string = map[string]string{
	"1NtL": db.A_OUT_TOTAL,
	"tP  ": db.G_TP,
	"EHtL": db.A_IN_TOTAL,
	"EHL1": db.A_IMPORT + "/0",
	"EHL2": db.A_IMPORT + "/1",
	"1NL1": db.A_EXPORT + "/0",
	"1NL2": db.A_EXPORT + "/1",
}

func init() {
	db.RegisterInit(meterReader)
}

func meterReader(d *db.DB) error {
	sect := d.Config.GetSection("meter")
	if sect == nil {
		return nil
	}
	var angle float64
	a, err := sect.GetArg("rotate")
	if err == nil {
		angle, err = strconv.ParseFloat(a, 64)
		if err != nil {
			return err
		}
	}
	source, err := sect.GetArg("source")
	if err != nil {
		return err
	}
	r, err := NewReader(sect, d.Trace)
	if err != nil {
		return err
	}
	d.AddGauge(db.G_TP)
	d.AddAccum(db.A_IN_TOTAL, false)
	d.AddAccum(db.A_OUT_TOTAL, false)
	d.AddSubAccum(db.A_IMPORT, false)
	d.AddSubAccum(db.A_IMPORT, false)
	d.AddSubAccum(db.A_EXPORT, false)
	d.AddSubAccum(db.A_EXPORT, false)
	log.Printf("Registered LCD decoder\n")
	go runReader(d, r, source, angle)
	return nil
}

func runReader(d *db.DB, r *Reader, source string, angle float64) {
	delay := time.Duration(*sampleTime) * time.Millisecond
	lastTime := time.Now()
	client := http.Client{
		Timeout: time.Duration(*sourceTimeout) * time.Second,
	}
	for {
		time.Sleep(delay - time.Now().Sub(lastTime))
		lastTime = time.Now()
		res, err := client.Get(source)
		if err != nil {
			log.Printf("Failed to retrieve source image from %s: %v", source, err)
			continue
		}
		img, _, err := image.Decode(res.Body)
		res.Body.Close()
		if err != nil {
			log.Printf("Failed to decode image from %s: %v", source, err)
			continue
		}
		if d.Trace {
			log.Printf("Successful image read from %s, delay %s", source, time.Now().Sub(lastTime).String())
		}
		if angle != 0 {
			img = lcd.RotateImage(img, angle)
		}
		label, val, err := r.Read(img)
		if err != nil {
			log.Printf("Read error: %v", err)
			if *saveBad {
				lcd.SaveImage(*badFile, img)
			}
		} else if len(label) > 0 {
			tag, ok := tagMap[label]
			if !ok {
				log.Printf("Unknown meter label: %s\n", label)
			} else {
				d.InChan <- db.Input{Tag: tag, Value: val}
			}
		}
		r.Recalibrate()
	}
}
