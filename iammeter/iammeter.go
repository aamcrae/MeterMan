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

// package iammeter polls a IAMMETER WEM3080 single phase energy meter.
// The package is configured as a section in the YAML config file:
//   iammeter:
//     meter: <url to retrieve data>
//     poll: <polling interval in seconds>
// e.g
// iammeter:
//   meter: http://admin:admin@meter-hostname/monitorjson
//   poll: 30
//
// The energy meter is polled, and the following values are extracted:
// Volts (V) -> G_VOLTS (averaged)
// +Current (A) -> G_IN_CURRENT
// -Current (A) -> G_OUT_CURRENT
// Power (kWh) -> G_IN_POWER, G_OUT_POWER
// Import (kWh) -> A_IN_TOTAL, A_IMPORT
// Export (kWh) -> A_OUT_TOTAL, A_EXPORT
// Optionally Frequency (Hertz) -> G_FREQ

package iammeter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lib"
)

type Iammeter struct {
	Meter  string
	Poll   int
	Offset int
}

const moduleName = "iammeter"

type imeter struct {
	d      *db.DB
	client http.Client
	url    string
	volts  string
	status string
}

// Register iamReader as a data source.
func init() {
	db.RegisterInit(iamReader)
}

// Set up polling the energy meter, if the config exists for it.
func iamReader(d *db.DB) error {
	var conf Iammeter
	c, ok := d.Config[moduleName]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	poll := lib.ConfigOrDefault(conf.Poll, 30)     // Default poll of 30 seconds
	offset := lib.ConfigOrDefault(conf.Offset, -5) // Default offset -5 seconds
	if len(conf.Meter) == 0 {
		return fmt.Errorf("iammeter: missing URL")
	}
	im := &imeter{d: d, url: conf.Meter, status: "init"}
	im.client = http.Client{
		Timeout: time.Duration(time.Second * 10), // 10 second timeout
	}
	im.d.AddStatusPrinter(moduleName, im.Status)
	log.Printf("Registered IAMMETER reader (polling interval %d seconds, offset %d)\n", poll, offset)
	if !d.Dryrun {
		im.volts = d.AddSubGauge(db.G_VOLTS, true)
		im.d.AddGauge(db.G_IN_CURRENT)
		im.d.AddGauge(db.G_OUT_CURRENT)
		im.d.AddGauge(db.G_IN_POWER)
		im.d.AddGauge(db.G_OUT_POWER)
		im.d.AddAccum(db.A_IN_TOTAL, true)
		im.d.AddAccum(db.A_OUT_TOTAL, true)
		im.d.AddAccum(db.A_IMPORT, true)
		im.d.AddAccum(db.A_EXPORT, true)
		im.d.AddGauge(db.G_FREQ)
		im.d.AddCallback(time.Second*time.Duration(poll), time.Second*time.Duration(offset), func(now time.Time) {
			go im.poll()
		})
	}
	return nil
}

func (im *imeter) Status() string {
	return im.status
}

func (im *imeter) poll() {
	err := im.fetch()
	if err != nil {
		log.Printf("iammeter: %v", err)
	}
}

func (im *imeter) fetch() error {
	type Top struct {
		Method  string    `json:"method"`
		Mac     string    `json:"mac"`
		Version string    `json:"version"`
		Server  string    `json:"server"`
		Serial  string    `json:"SN"`
		Data    []float64 `json:"Data"`
	}
	var b strings.Builder
	defer func() { im.status = b.String() }()
	fmt.Fprintf(&b, "%s: ", time.Now().Format("2006-01-02 15:04"))
	resp, err := im.client.Get(im.url)
	if err != nil {
		fmt.Fprintf(&b, "Get: %v", err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(&b, "ReadAll: %v", err)
		return err
	}
	var m Top
	err = json.Unmarshal(body, &m)
	if err != nil {
		fmt.Fprintf(&b, "Unmarshal: %v", err)
		return err
	}
	if !(len(m.Data) == 5 || len(m.Data) == 7) {
		fmt.Fprintf(&b, "Malformed data from meter")
		return fmt.Errorf("malformed data from meter")
	}
	fmt.Fprintf(&b, "OK - Volts: %s, current %s, power %s", lib.FmtFloat(m.Data[0]), lib.FmtFloat(m.Data[1]), lib.FmtFloat(m.Data[2]))
	fmt.Fprintf(&b, ", Imp: %s, Exp %s", lib.FmtFloat(m.Data[3]), lib.FmtFloat(m.Data[4]))
	if len(m.Data) == 7 {
		fmt.Fprintf(&b, ", Frequency: %s, factor %s", lib.FmtFloat(m.Data[5]), lib.FmtFloat(m.Data[6]))
	}
	if im.d.Trace {
		log.Printf("iammeter: version %s, serial number %s", m.Version, m.Serial)
		log.Printf("iammeter: Volts %gV, current %gA, power %gW", m.Data[0], m.Data[1], m.Data[2])
		log.Printf("iammeter: Import %gkWh, Export %gkWh", m.Data[3], m.Data[4])
		if len(m.Data) == 7 {
			log.Printf("iammeter: Frequency %gHz, Power factor %g", m.Data[5], m.Data[6])
		}
	}
	if len(m.Version) == 0 || len(m.Serial) == 0 || m.Data[0] == 0 ||
		m.Data[3] == 0 || m.Data[4] == 0 {
		return fmt.Errorf("Missing values")
	}
	im.d.Input(im.volts, m.Data[0])
	if m.Data[2] < 0.0 {
		im.d.Input(db.G_IN_CURRENT, 0.0)
		im.d.Input(db.G_IN_POWER, 0.0)
		im.d.Input(db.G_OUT_CURRENT, m.Data[1])
		im.d.Input(db.G_OUT_POWER, m.Data[2]/-1000.0)
	} else {
		im.d.Input(db.G_OUT_POWER, 0.0)
		im.d.Input(db.G_OUT_CURRENT, 0.0)
		im.d.Input(db.G_IN_CURRENT, m.Data[1])
		im.d.Input(db.G_IN_POWER, m.Data[2]/1000.0)
	}
	// ImportEnergy
	im.d.Input(db.A_IN_TOTAL, m.Data[3])
	im.d.Input(db.A_IMPORT, m.Data[3])
	// ExportGrid
	im.d.Input(db.A_OUT_TOTAL, m.Data[4])
	im.d.Input(db.A_EXPORT, m.Data[4])
	if len(m.Data) == 7 {
		im.d.Input(db.G_FREQ, m.Data[5])
	}
	return nil
}
