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
	"fmt"
	"image"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/lcd"
	"github.com/aamcrae/config"
)

var recalInterval = flag.Int("recal_interval", 120, "Recalibrate interval (seconds)")
var recalibrate = flag.Bool("recalibrate", false, "Recalibrate with new image")
var saveCalibration = flag.Bool("save_calibration", false, "Save calibration data")

type limit struct {
	last  time.Time
	value float64
}

type Reader struct {
	trace           bool
	decoder         *lcd.LcdDecoder
	current         image.Image
	lastCalibration time.Time
	limits          map[string]limit
	calFile         string
}

type measure struct {
	handler func(*Reader, *measure, string, string) (string, float64, error)
	scale   float64
	min     float64 // Valid minimum
	max     float64 // Valid maximum (or hourly max increase)
}

var measures map[string]*measure = map[string]*measure{
	"1nP1": &measure{handlerIgnore, 1.0, 0, 0},
	"1nP2": &measure{handlerIgnore, 1.0, 0, 0},
	"t1NE": &measure{handlerIgnore, 1.0, 0, 0},
	"1NtL": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"tP  ": &measure{handlerNumber, 10000.0, 0, 0}, // Kw
	"EHtL": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"EHL1": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"EHL2": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"1NL1": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"1NL2": &measure{handlerAccum, 100.0, 0, 0},    // KwH
	"8888": &measure{handlerCalibrate, 1.0, 0, 0},
}

func NewReader(c *config.Section, trace bool) (*Reader, error) {
	d, err := lcd.CreateLcdDecoder(c)
	if err != nil {
		return nil, err
	}
	for _, e := range c.Get("range") {
		if len(e.Tokens) != 3 {
			return nil, fmt.Errorf("Bad 'range' parameters at %s:%d", e.Filename, e.Lineno)
		}
		m, ok := measures[e.Tokens[0]]
		if !ok {
			return nil, fmt.Errorf("Unknown measurement (%s) at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		min, err := strconv.ParseFloat(e.Tokens[1], 64)
		if err != nil {
			return nil, fmt.Errorf("Ilegal min value (%s) at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		max, err := strconv.ParseFloat(e.Tokens[2], 64)
		if err != nil {
			return nil, fmt.Errorf("Ilegal max value (%s) at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		m.min = min
		m.max = max
		if trace {
			log.Printf("Setting range of '%s' to [%f, %f]\n", e.Tokens[0], min, max)
		}
	}
	cf, err := c.GetArg("calibration")
	r := &Reader{trace: trace, decoder: d, limits: map[string]limit{}, calFile: cf}
	r.Restore()
	r.lastCalibration = time.Now()
	s, err := c.GetArg("calibrate")
	if err == nil {
		img, err := lcd.ReadImage(s)
		if err != nil {
			return nil, err
		}
		r.decoder.CalibrateImage(img, "888888888888")
	}
	return r, nil
}

// Restore any saved calibration.
func (r *Reader) Restore() {
	if len(r.calFile) != 0 {
		if f, err := os.Open(r.calFile); err != nil {
			log.Printf("%s: %v\n", r.calFile, err)
		} else {
			r.decoder.RestoreCalibration(f)
			f.Close()
			r.decoder.PickCalibration()
		}
	}
}

// Save the current calibration.
func (r *Reader) Save() {
	if *saveCalibration && len(r.calFile) != 0 {
		if r.trace {
			log.Printf("Saving calibration data")
		}
		if f, err := os.Create(r.calFile); err != nil {
			log.Printf("Calibration file %s: %v\n", r.calFile, err)
		} else {
			r.decoder.SaveCalibration(f)
			err := f.Close()
			if err != nil {
				log.Printf("Save calibration: %s: %v\n", r.calFile, err)
			}
		}
	}
}

// A successful scan is used to adjust the scan levels.
func (r *Reader) GoodScan(res *lcd.ScanResult) {
	r.decoder.Good()
	if *recalibrate {
		err := r.decoder.CalibrateScan(res)
		if err != nil {
			log.Printf("CalibrateScan error: %v\n", err)
		}
	}
}

// If enabled, save the calibration data and recalibrate.
func (r *Reader) Recalibrate() {
	if *recalibrate {
		// Regularly, save the calibration data.
		now := time.Now()
		if time.Now().Sub(r.lastCalibration) >= time.Duration(*recalInterval)*time.Second {
			r.lastCalibration = now
			r.decoder.Recalibrate()
			r.Save()
		}
	}
}

func (r *Reader) Read(img image.Image) (string, float64, error) {
	r.current = img
	res := r.decoder.Decode(img)
	if res.Invalid > 0 {
		var badSeg []string
		for s := range res.Digits {
			if !res.Digits[s].Valid {
				badSeg = append(badSeg, fmt.Sprintf("%d[%02x]", s, res.Digits[s].Mask))
			}
		}
		r.decoder.Bad()
		return "", 0.0, fmt.Errorf("Bad read on segment[s] %s", strings.Join(badSeg, ","))
	}
	key := res.Text[0:4]
	value := res.Text[4:]
	m, ok := measures[key]
	if !ok {
		r.decoder.Bad()
		return "", 0.0, fmt.Errorf("Unknown key (%s) value %s", key, value)
	}
	str, num, err := m.handler(r, m, key, value)
	if err == nil {
		r.GoodScan(res)
	} else {
		r.decoder.Bad()
	}
	return str, num, err
}

func handlerIgnore(r *Reader, m *measure, key, value string) (string, float64, error) {
	return "", 0.0, nil
}

func handlerNumber(r *Reader, m *measure, key, value string) (string, float64, error) {
	v, err := r.getNumber(m, value)
	if err != nil {
		return "", 0, fmt.Errorf("key %s: %v", key, err)
	}
	if v < m.min || v >= m.max {
		return "", 0, fmt.Errorf("%s Out of range (%f), min %f, max %f", key, v, m.min, m.max)
	}
	if r.trace {
		log.Printf("Meter read: key %s value %f, min %f, max %f\n", key, v, m.min, m.max)
	}
	return key, v, nil
}

func handlerAccum(r *Reader, m *measure, key, value string) (string, float64, error) {
	v, err := r.getNumber(m, value)
	if err != nil {
		return "", 0, fmt.Errorf("key %s: %v", key, err)
	}
	lv, ok := r.limits[key]
	now := time.Now()
	if ok {
		diff := (v - lv.value) * 3600 / now.Sub(lv.last).Seconds()
		if diff < 0 {
			return "", 0.0, fmt.Errorf("%s Going backwards (old %f, new %f)", key, lv.value, v)
		}
		// Calculate and compare hourly change.
		if diff > m.max {
			return "", 0.0, fmt.Errorf("%s limit exceeded (old %f, change = %f, limit = %f)", key, lv.value, diff, m.max)
		}
		if r.trace {
			log.Printf("Meter read: key %s value %f, change %f, max %f\n", key, v, diff, m.max)
		}
	}
	r.limits[key] = limit{now, v}
	return key, v, nil
}

func handlerCalibrate(r *Reader, m *measure, key, value string) (string, float64, error) {
	if value != "88888888" {
		return "", 0.0, fmt.Errorf("Wrong calibration value (%s)", value)
	}
	return "", 0.0, nil
}

func (r *Reader) getNumber(m *measure, value string) (float64, error) {
	sv := value
	scale := m.scale
	if sv[0] == '-' {
		scale = -scale
		sv = sv[1:]
	}
	v, err := strconv.ParseFloat(strings.Trim(sv, " "), 64)
	if err != nil {
		return 0, fmt.Errorf("%s: bad number (%v)\n", value, err)
	}
	return v / scale, nil
}
