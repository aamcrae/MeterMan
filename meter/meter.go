// Copyright 2019 Google LLC // // Licensed under the Apache License, Version 2.0 (the "License"); // you may not use this file except in compliance with the License.  // You may obtain a copy of the License at
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
// The package is configured as a section in the YAML config file:
//   meter:
//     source: <url to retrieve image>
//     rotate: <degrees to rotate clockwise> # Optional
//     threshold: <threshold percentage> # Optional
//     calibrate: <image file to be used for initial calibration> # Optional
//     calibration: <cache file to store moving calibrations>
//     # Optional offset for offsetting all digits.
//     offset: [<X offset, Y offset>]
//     # Top left (TL) is assumed 0,0
//     # TR = Top right,BR - bottom right etc.
//     lcd:
//       - name: <digit name>
//         tr: [x,y]
//         br: [x,y]
//         bl: [x,y]
//         width: <line width>
//       ...
//     digit:
//       - lcd: <digit name>
//         coord: [x,y]  # co-ordinate of TL corner
//       ...
//     # Optional limits for meter key.
//     range:
//       - key: <key>
//         min: <min>
//         max: <max>
//       ...

package meter

import (
	"image"
	"log"
	"net/http"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lib"
	"github.com/aamcrae/lcd"
)

type MeterConfig struct {
	Source        string
	Rotate        float64
	lcd.LcdConfig `yaml:",inline"`
	Range         []struct {
		Key string
		Min float64
		Max float64
	}
	BadImage            string // Save bad image to this file.
	Timeout             int    // Timeout in seconds
	Sample              int    // Sample interval in milliseconds
	History             int    // Size of moving average cache
	MaxLevels           int    // Size of calibration level map
	SavedLevels         int    // Number of levels saved
	Recalibrate         bool   // Recalibrate
	Calibration         string // Calibration save file
	RecalibrateInterval int    // Recalibrate interval in seconds
}

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
	var conf MeterConfig
	yaml, ok := d.Config["meter"]
	if !ok {
		return nil
	}
	err := yaml.Decode(&conf)
	if err != nil {
		return err
	}
	r, err := NewReader(&conf, d.Trace)
	if err != nil {
		return err
	}
	d.AddDiff(db.D_IN_POWER, time.Minute*5)
	d.AddDiff(db.D_OUT_POWER, time.Minute*5)
	d.AddAccum(db.A_IN_TOTAL, true)
	d.AddAccum(db.A_OUT_TOTAL, true)
	d.AddSubAccum(db.A_IMPORT, true)
	d.AddSubAccum(db.A_IMPORT, true)
	d.AddSubAccum(db.A_EXPORT, true)
	d.AddSubAccum(db.A_EXPORT, true)
	log.Printf("Registered meter LCD reader (%d digits)\n", len(conf.Digit))
	if !d.Dryrun {
		go runReader(d, r, &conf)
	}
	return nil
}

// runReader is a loop that reads the image of the meter panel
// from an image source, and decodes the LCD digits.
func runReader(d *db.DB, r *Reader, conf *MeterConfig) {
	delay := time.Millisecond * time.Duration(lib.ConfigOrDefault(conf.Sample, 4900))            // Sample rate in milliseconds
	timeout := time.Second * time.Duration(lib.ConfigOrDefault(time.Duration(conf.Timeout), 20)) // Timeout in seconds
	lastTime := time.Now()
	client := http.Client{
		Timeout: timeout,
	}
	for {
		time.Sleep(delay - time.Now().Sub(lastTime))
		lastTime = time.Now()
		res, err := client.Get(conf.Source)
		if err != nil {
			log.Printf("Failed to retrieve source image from %s: %v", conf.Source, err)
			continue
		}
		img, _, err := image.Decode(res.Body)
		res.Body.Close()
		if err != nil {
			log.Printf("Failed to decode image from %s: %v", conf.Source, err)
			continue
		}
		if d.Trace {
			log.Printf("Successful image read from %s, delay %s", conf.Source, time.Now().Sub(lastTime).String())
		}
		if conf.Rotate != 0 {
			img = lcd.RotateImage(img, conf.Rotate)
		}
		// Decode the digits and get the label and value.
		label, val, err := r.Read(img)
		if err != nil {
			if d.Trace {
				log.Printf("Read error: %v", err)
			}
			if len(conf.BadImage) != 0 {
				log.Printf("Read error: %v, image saved to %s", err, conf.BadImage)
				lcd.SaveImage(conf.BadImage, img)
			}
		} else if len(label) > 0 {
			tags, ok := tagMap[label]
			if !ok {
				log.Printf("Unknown meter label: %s\n", label)
			} else {
				for _, tag := range tags {
					d.Input(tag, val)
				}
			}
		}
		// If required, recalibrate the reader.
		r.Recalibrate()
	}
}
