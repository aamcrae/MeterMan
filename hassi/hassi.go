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

// package hassi implements a writer that uploads data
// to the Home Assistant API.
//
// The package is configured as a section in the main config file
// under the '[hassi]' section, and the parameters are:
//  [hassi]
//  apikey=<apikey from Home Assistant>
//  url=<API endpoint>
//
// Values that are not stale are sent to Home assistant.

package hassi

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var hassiRate = flag.Int("hassirate", 2, "Home Assistant update rate (in minutes)")

type hassi struct {
	d      *db.DB
	url    string
	key    string
	client *http.Client
}

func init() {
	db.RegisterInit(hassiInit)
}

func hassiInit(d *db.DB) error {
	sect := d.Config.GetSection("hassi")
	if sect == nil {
		return nil
	}
	key, err := sect.GetArg("apikey")
	if err != nil {
		return err
	}
	url, err := sect.GetArg("url")
	if err != nil {
		return err
	}
	key = fmt.Sprintf("Bearer %s", key)
	h := &hassi{d: d, url: url, key: key, client: &http.Client{}}
	intv := time.Minute * time.Duration(*hassiRate)
	d.AddCallback(h, intv)
	log.Printf("Registered Home Assistant uploader (%s interval)\n", intv)
	return nil
}

// Run uploads any updated tags to Home Assistant.
func (h *hassi) Run(last time.Time, now time.Time) {
	type blk struct {
		State string             `json:"state"`
		Attr  map[string]float64 `json:"attributes"`
	}
	var b blk
	b.Attr = make(map[string]float64)
	h.add(db.D_IN_POWER, "in_power", last, b.Attr)
	h.add(db.D_OUT_POWER, "out_power", last, b.Attr)
	in_p := h.d.GetElement(db.D_IN_POWER)
	out_p := h.d.GetElement(db.D_OUT_POWER)
	if isFresh(in_p, last) && isFresh(out_p, last) {
		b.Attr["meter_power"] = in_p.Get() - out_p.Get()
		if in_p.Get() <= out_p.Get() {
			b.State = "exporting"
		} else {
			b.State = "importing"
		}
	}
	h.add(db.G_VOLTS, "volts", last, b.Attr)
	h.add(db.D_GEN_P, "gen_power", last, b.Attr)
	h.daily(db.A_OUT_TOTAL, "out", last, b.Attr)
	h.daily(db.A_IN_TOTAL, "in", last, b.Attr)
	h.daily(db.A_GEN_TOTAL, "gen_daily", last, b.Attr)
	h.daily(db.A_IMPORT, "import", last, b.Attr)
	h.daily(db.A_EXPORT, "export", last, b.Attr)
	// Send request asynchronously.
	go func() {
		buf := new(bytes.Buffer)
		json.NewEncoder(buf).Encode(&b)
		req, err := http.NewRequest("POST", h.url, buf)
		if err != nil {
			log.Printf("NewRequest (%s) failed: %v", h.url, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", h.key)
		res, err := h.client.Do(req)
		if err != nil {
			log.Printf("Req (%s) failed: %v", h.url, err)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != 200 && res.StatusCode != 201 {
			log.Printf("hassi: req %s, resp %s", h.url, res.Status)
		}
		if h.d.Trace {
			log.Printf("hassi: Sent req %s, resp %s", h.url, res.Status)
		}
	}()
}

func (h *hassi) add(tag, attr string, last time.Time, m map[string]float64) bool {
	e := h.d.GetElement(tag)
	if isFresh(e, last) {
		m[attr] = e.Get()
		return true
	}
	return false
}

func (h *hassi) daily(tag, attr string, last time.Time, m map[string]float64) {
	e := h.d.GetAccum(tag)
	if isFresh(e, last) {
		m[attr] = e.Daily()
	}
}

// isFresh will return true if the element is not nil and has been updated
// in the last interval.
func isFresh(e db.Element, last time.Time) bool {
	return e != nil && !e.Timestamp().Before(last)
}
