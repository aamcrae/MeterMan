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

// package meter uses the lcd package to read and decode a meter
// LCD image. The LCD format that is supported is:
//     kkkkNNNNNNNN
// where kkkk is a 4 character key, and N..N is an 8 digit number.
// The package is configured as a section in the main config file
// under the '[meter]' section, and the parameters are:
//   [meter]
//   source=<url to retrieve image>
//   rotate=<degrees to rotate clockwise> # Optional
//   threshold=<threshold percentage> # Optional
//   calibrate=<image file to be used for initial calibration> # Optional
//   calibration=<cache file to store moving calibrations>
//   # Optional offset for offsetting all digits.
//   offset=<X offset, Y offset>
//   # Top left (TL) is assumed 0,0
//   # TR = Top right,BR - bottom right etc.
//   lcd=<digit name>,<X TR>,<Y TR>,<X BR>,<Y BR>,<X BL>,<Y BL>,<line width>
//   digit=<digit name>,<X>,<Y>  # co-ordinate of TL corner
//   # Optional limits for meter key.
//   range=<key>,<min>,<max>

package meter

import (
	"flag"
	"image"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aamcrae/lcd"
	"github.com/aamcrae/MeterMan/db"
)

var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")
var sampleTime = flag.Int("sample", 4900, "Image sample rate (milliseconds)")
var sourceTimeout = flag.Int("source_timeout", 20, "Source timeout in seconds")

// Maps meter label to database tag.
var tagMap map[string][]string = map[string][]string{
	"1NtL": {db.A_OUT_TOTAL, db.D_OUT_POWER},
	"EHtL": {db.A_IN_TOTAL, db.D_IN_POWER},
	"EHL1": {db.A_IMPORT + "/0"},
	"EHL2": {db.A_IMPORT + "/1"},
	"1NL1": {db.A_EXPORT + "/0"},
	"1NL2": {db.A_EXPORT + "/1"},
}

// Register meterReader as a data source.
func init() {
	db.RegisterInit(meterReader)
}

// Create an instance of a meter reader, if there is
// a config section declared for it.
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
	d.AddDiff(db.D_IN_POWER)
	d.AddDiff(db.D_OUT_POWER)
	d.AddAccum(db.A_IN_TOTAL, true)
	d.AddAccum(db.A_OUT_TOTAL, true)
	d.AddSubAccum(db.A_IMPORT, true)
	d.AddSubAccum(db.A_IMPORT, true)
	d.AddSubAccum(db.A_EXPORT, true)
	d.AddSubAccum(db.A_EXPORT, true)
	log.Printf("Registered meter LCD reader\n")
	go runReader(d, r, source, angle)
	return nil
}

// runReader is a loop that reads the image of the meter panel
// from an image source, and decodes the LCD digits.
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
		// Decode the digits and get the label and value.
		label, val, err := r.Read(img)
		if err != nil {
			log.Printf("Read error: %v", err)
			if *saveBad {
				lcd.SaveImage(*badFile, img)
			}
		} else if len(label) > 0 {
			tags, ok := tagMap[label]
			if !ok {
				log.Printf("Unknown meter label: %s\n", label)
			} else {
				for _, tag := range tags {
					d.InChan <- db.Input{Tag: tag, Value: val}
				}
			}
		}
		// If required, recalibrate the reader.
		r.Recalibrate()
	}
}
