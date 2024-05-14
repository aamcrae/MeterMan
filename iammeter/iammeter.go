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
// Power (kWh) -> ignored
// Import (kWh) -> D_IN_POWER, A_IN_TOTAL, A_IMPORT
// Export (kWh) -> D_OUT_POWER, A_OUT_TOTAL, A_EXPORT

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
	Meter string
	Poll  int
}
const defaultPoll = 15 // Default poll interval in seconds

const moduleName = "iammeter"

type imeter struct {
	d	  *db.DB
	client http.Client
	url	string
	volts string
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
	poll := lib.ConfigOrDefault(conf.Poll, defaultPoll)
	if len(conf.Meter) == 0 {
		return fmt.Errorf("iammeter: missing URL")
	}
	im := &imeter{d: d, url: conf.Meter, status: "init"}
	im.client = http.Client{
		Timeout: time.Duration(time.Second * 10),
	}
	im.d.AddStatusPrinter(moduleName, im.Status)
	log.Printf("Registered IAMMETER reader (polling interval %d seconds)\n", poll)
	if !d.Dryrun {
		im.volts = d.AddSubGauge(db.G_VOLTS, true)
		im.d.AddGauge(db.G_IN_CURRENT)
		im.d.AddGauge(db.G_OUT_CURRENT)
		im.d.AddDiff(db.D_IN_POWER, time.Minute*5)
		im.d.AddDiff(db.D_OUT_POWER, time.Minute*5)
		im.d.AddAccum(db.A_IN_TOTAL, true)
		im.d.AddAccum(db.A_OUT_TOTAL, true)
		im.d.AddAccum(db.A_IMPORT, true)
		im.d.AddAccum(db.A_EXPORT, true)
		go im.meterReader(time.Duration(poll)*time.Second)
	}
	return nil
}

// meterReader is a loop that reads the data from the energy meter.
func (im *imeter) meterReader(delay time.Duration) {
	lastTime := time.Now()
	for {
		time.Sleep(delay - time.Now().Sub(lastTime))
		lastTime = time.Now()
		err := im.fetch()
		if err != nil {
			log.Printf("iammeter: %v", err)
		}
	}
}

func (im *imeter) Status() string {
	return im.status
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
	defer func() {im.status = b.String()}()
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
	if len(m.Data) != 5 {
		fmt.Fprintf(&b, "Malformed data from meter")
		return fmt.Errorf("malformed data from meter")
	}
	fmt.Fprintf(&b, "OK - Volts: %s, current %s, power %s", lib.FmtFloat(m.Data[0]), lib.FmtFloat(m.Data[1]), lib.FmtFloat(m.Data[2]))
	fmt.Fprintf(&b, ", Imp: %s, Exp %s", lib.FmtFloat(m.Data[3]), lib.FmtFloat(m.Data[4]))
	if im.d.Trace {
		log.Printf("iammeter: version %s, serial number %s", m.Version, m.Serial)
		log.Printf("iammeter: Volts %gV, current %gA, power %gW", m.Data[0], m.Data[1], m.Data[2])
		log.Printf("iammeter: Import %gkWh, Export %gkWh", m.Data[3], m.Data[4])
	}
	if len(m.Version) == 0 || len(m.Serial) == 0 || m.Data[0] == 0 ||
		m.Data[3] == 0 || m.Data[4] == 0 {
		return fmt.Errorf("Missing values")
	}
	im.d.Input(im.volts, m.Data[0])
	if m.Data[1] < 0.0 {
		im.d.Input(db.G_IN_CURRENT, 0.0)
		im.d.Input(db.G_OUT_CURRENT, -m.Data[1])
	} else {
		im.d.Input(db.G_OUT_CURRENT, 0.0)
		im.d.Input(db.G_IN_CURRENT, m.Data[1])
	}
	// ImportEnergy
	im.d.Input(db.D_IN_POWER, m.Data[3])
	im.d.Input(db.A_IN_TOTAL, m.Data[3])
	im.d.Input(db.A_IMPORT, m.Data[3])
	// ExportGrid
	im.d.Input(db.D_OUT_POWER, m.Data[4])
	im.d.Input(db.A_OUT_TOTAL, m.Data[4])
	im.d.Input(db.A_EXPORT, m.Data[4])
	return nil
}
