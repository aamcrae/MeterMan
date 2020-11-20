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
	"strconv"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/lcd"
	"github.com/aamcrae/MeterMan/lib"
	"github.com/aamcrae/config"
)

var levelSize = flag.Int("level_size", 0, "Size of calibration level map")
var savedLevels = flag.Int("level_saved", 50, "Number of levels saved")
var history = flag.Int("history", 0, "Size of moving average cache")
var recalInterval = flag.Int("recal_interval", 120, "Recalibrate interval (seconds)")
var recalibrate = flag.Bool("recalibrate", false, "Recalibrate with new image")
var saveCalibration = flag.Bool("save_calibration", false, "Save calibration data")

// limit holds the last value read, along with the time.
// Used to sanity check accumulator values.
type limit struct {
	last  time.Time
	value float64
}

// Reader is a meter specific reader.
type Reader struct {
	trace           bool
	decoder         *lcd.LcdDecoder
	current         image.Image
	lastCalibration time.Time
	limits          map[string]limit
	calFile         string
}

// measure represents one type of value decoded from the meter.
type measure struct {
	handler func(*Reader, *measure, string, string) (string, float64, error)
	scale   float64
	min     float64 // Valid minimum
	max     float64 // Valid maximum (or hourly max increase)
}

// The label string decoded from the meter maps the data
// to a label specific handler.
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
	"8888": &measure{handlerCalibrate, 1.0, 0, 0},  // all segments on
	"4613": &measure{handlerUpdate, 1.0, 0, 0},
}

// Creates a new reader.
func NewReader(c *config.Section, trace bool) (*Reader, error) {
	d, err := lcd.CreateLcdDecoder(c)
	if *history > 0 {
		d.History = *history
	}
	if *levelSize > 0 {
		d.MaxLevels = *levelSize
	}
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
	if len(r.calFile) != 0 {
		n, err := r.decoder.RestoreFromFile(r.calFile)
		if err != nil {
			return nil, err
		}
		if r.trace {
			log.Printf("Restored %d calibration entries from %s", n, r.calFile)
		}
	}
	r.lastCalibration = time.Now()
	s, err := c.GetArg("calibrate")
	if err == nil {
		img, err := lib.ReadImage(s)
		if err != nil {
			return nil, err
		}
		r.decoder.Preset(img, "888888888888")
	}
	return r, nil
}

// The image was successfully scanned and decoded, at least
// within the heuristics that are possible. There is no guarantee
// that the decode was completely correct e.g it is possible to
// misread one or more digits such as mistaking a 1 for a 7.
// Given that this is considered a successful decode, use the
// levels that were sampled in this image to adjust the calibration
// levels being used in the decoder. This allows the decoder to
// dynamically adjust to changing image conditions.
func (r *Reader) GoodScan(res *lcd.DecodeResult) {
	r.decoder.Good()
	if *recalibrate {
		err := r.decoder.CalibrateUsingScan(res.Img, res.Scans)
		if err != nil {
			log.Printf("CalibrateUsingScan: %v\n", err)
		}
	}
}

// If enabled, save the calibration data and recalibrate.
func (r *Reader) Recalibrate() {
	if *recalibrate {
		// Regularly, save the calibration data.
		now := time.Now()
		if time.Now().Sub(r.lastCalibration) >= time.Duration(*recalInterval)*time.Second {
			l := r.decoder
			r.lastCalibration = now
			l.Recalibrate()
			log.Printf("Recalibration: last %3d (good %2d, bad %2d), new %3d, worst %3d, count %d, avg %5.1f",
				l.LastQuality, l.LastGood, l.LastBad, l.Best, l.Worst, l.Count, float32(l.Total)/float32(l.Count))
			if *saveCalibration && len(r.calFile) != 0 {
				if r.trace {
					log.Printf("Saving calibration data to %s", r.calFile)
				}
				err := l.SaveToFile(r.calFile, *savedLevels)
				if err != nil {
					log.Printf("%s: %v\n", r.calFile, err)
				}
			}
		}
	}
}

// Scan and decode the digits in the image.
func (r *Reader) Read(img image.Image) (string, float64, error) {
	r.current = img
	res := r.decoder.Decode(img)
	// Check for invalid digits.
	if res.Invalid > 0 {
		var badSeg []string
		for s := range res.Decodes {
			if !res.Decodes[s].Valid {
				badSeg = append(badSeg, fmt.Sprintf("%d[%02x]", s, res.Scans[s].Mask))
			}
		}
		r.decoder.Bad()
		return "", 0.0, fmt.Errorf("Bad read on segment[s] %s", strings.Join(badSeg, ","))
	}
	// Valid characters were obtained from the image. Check these against
	// the expected labels and digits.
	if r.trace {
		log.Printf("LCD image processed: text=<%s>", res.Text)
	}
	key := res.Text[0:4]
	value := res.Text[4:]
	m, ok := measures[key]
	if !ok {
		// Even though characters were successfully decoded from the image,
		// the label does not match any expected strings, so this is
		// marked as a misread.
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

// A valid label, but we are not interested in the value.
func handlerIgnore(r *Reader, m *measure, key, value string) (string, float64, error) {
	if r.trace {
		log.Printf("Meter read: ignoring key %s, value %s", key, value)
	}
	return "", 0.0, nil
}

// The label identifies a number we are interested in.
// Scan for a number and sanity check it.
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

// The label identifies a number that is an accumulator i.e
// a value that is increasing. Check that the value is only increasing, and that
// the increment is not more than the maximum defined.
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
			log.Printf("%s Going backwards (old %f, new %f)", key, lv.value, v)
			diff = -diff
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

// All segments are set.
func handlerCalibrate(r *Reader, m *measure, key, value string) (string, float64, error) {
	if value != "88888888" {
		return "", 0.0, fmt.Errorf("Wrong calibration value (%s)", value)
	}
	return "", 0.0, nil
}

func handlerUpdate(r *Reader, m *measure, key, value string) (string, float64, error) {
	if value != "CoNPLEtE" {
		return "", 0.0, fmt.Errorf("Wrong update value (%s)", value)
	}
	return "", 0.0, nil
}

// Decode the latter part of the scanned string as a number.
// The value may be negative ('-' as the first character).
// A scale is applied to convert the number.
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
