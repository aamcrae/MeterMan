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
//  update=60 # Update rate in seconds
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
	"strconv"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var hassiRate = flag.Int("hassirate", 120, "Default Home Assistant update rate (seconds)")

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
	rate := *hassiRate
	hr, err := sect.GetArg("update")
	if err == nil {
		if v, err := strconv.ParseInt(hr, 10, 32); err != nil {
			return fmt.Errorf("hassi update value error: %v", err)
		} else {
			rate = int(v)
		}
	}
	key = fmt.Sprintf("Bearer %s", key)
	h := &hassi{d: d, url: url, key: key, client: &http.Client{}}
	intv := time.Second * time.Duration(rate)
	d.AddCallback(intv, h.send)
	log.Printf("Registered Home Assistant uploader (%d seconds interval)", rate)
	return nil
}

// Upload any updated tags to Home Assistant.
func (h *hassi) send(now time.Time) {
	type blk struct {
		State string             `json:"state"`
		Attr  map[string]float64 `json:"attributes"`
	}
	var b blk
	b.Attr = make(map[string]float64)
	h.add(db.D_IN_POWER, "in_power", b.Attr)
	h.add(db.D_OUT_POWER, "out_power", b.Attr)
	in_p := h.d.GetElement(db.D_IN_POWER)
	out_p := h.d.GetElement(db.D_OUT_POWER)
	if in_p.Fresh() && out_p.Fresh() {
		b.Attr["meter_power"] = in_p.Get() - out_p.Get()
		if in_p.Get() <= out_p.Get() {
			b.State = "exporting"
		} else {
			b.State = "importing"
		}
	}
	h.add(db.G_VOLTS, "volts", b.Attr)
	h.add(db.D_GEN_P, "gen_power", b.Attr)
	h.daily(db.A_OUT_TOTAL, "out", b.Attr)
	h.daily(db.A_IN_TOTAL, "in", b.Attr)
	h.daily(db.A_GEN_TOTAL, "gen", b.Attr)
	h.daily(db.A_IMPORT, "import", b.Attr)
	h.daily(db.A_EXPORT, "export", b.Attr)
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

func (h *hassi) add(tag, attr string, m map[string]float64) bool {
	e := h.d.GetElement(tag)
	if e.Fresh() {
		m[attr] = e.Get()
		return true
	}
	return false
}

func (h *hassi) daily(tag, attr string, m map[string]float64) {
	e := h.d.GetAccum(tag)
	if e.Fresh() {
		m[attr+"_daily"] = e.Daily()
		m[attr+"_total"] = e.Get()
	}
}
