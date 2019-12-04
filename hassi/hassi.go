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

// package hassi implements a writer that uploads selected data
// to the Home Assistant API.
//
// The package is configured as a section in the main config file
// under the '[hassi]' section, and the parameters are:
//  [hassi]
//  apikey=<apikey from Home Assistant>
//  url=<API endpoint>
//  # An entity entry for each tag to upload.
//  entity=<tag>,<Home Assistant entity ID>

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

var hassiRate = flag.Int("hassirate", 1, "Home Assistant update rate (in minutes)")

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
	d.AddUpdate(h, intv)
	log.Printf("Registered Home Assistant uploader (%s interval)\n", intv)
	return nil
}

// Update uploads any updated tags to Home Assistant.
func (h *hassi) Update(last time.Time, now time.Time) {
	type blk struct {
		State string             `json:"state"`
		Attr  map[string]float64 `json:"attributes"`
	}
	var b blk
	tp := h.d.GetElement("TP")
	if isFresh(tp, last) && tp.Get() < 0 {
		b.State = "exporting"
	} else {
		b.State = "importing"
	}
	b.Attr = make(map[string]float64)
	h.add(db.G_TP, "meter_power", last, b.Attr)
	h.add(db.G_VOLTS, "volts", last, b.Attr)
	h.add(db.G_GEN_P, "gen_power", last, b.Attr)
	h.daily(db.A_OUT_TOTAL, "out", last, b.Attr)
	h.daily(db.A_IN_TOTAL, "in", last, b.Attr)
	h.daily(db.A_GEN_TOTAL, "gen_daily", last, b.Attr)
	h.daily(db.A_IMPORT, "import", last, b.Attr)
	h.daily(db.A_EXPORT, "export", last, b.Attr)
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
}

func (h *hassi) add(tag, attr string, last time.Time, m map[string]float64) {
	e := h.d.GetElement(tag)
	if isFresh(e, last) {
		m[attr] = e.Get()
	}
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
