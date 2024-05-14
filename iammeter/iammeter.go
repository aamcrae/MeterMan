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
	"time"

	"github.com/aamcrae/MeterMan/db"
)

type Iammeter struct {
	Meter string
	Poll  int
}

const defaultPoll = 15 // Default poll interval in seconds

// Register iamReader as a data source.
func init() {
	db.RegisterInit(iamReader)
}

// Set up polling the energy meter, if the config exists for it.
func iamReader(d *db.DB) error {
	var conf Iammeter
	c, ok := d.Config["iammeter"]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	poll := db.ConfigOrDefault(conf.Poll, defaultPoll)
	if len(conf.Meter) == 0 {
		return fmt.Errorf("iammeter: missing URL")
	}
	log.Printf("Registered IAMMETER reader (polling interval %d seconds)\n", poll)
	if !d.Dryrun {
		vg := d.AddSubGauge(db.G_VOLTS, true)
		d.AddGauge(db.G_IN_CURRENT)
		d.AddGauge(db.G_OUT_CURRENT)
		d.AddDiff(db.D_IN_POWER, time.Minute*5)
		d.AddDiff(db.D_OUT_POWER, time.Minute*5)
		d.AddAccum(db.A_IN_TOTAL, true)
		d.AddAccum(db.A_OUT_TOTAL, true)
		d.AddAccum(db.A_IMPORT, true)
		d.AddAccum(db.A_EXPORT, true)
		go meterReader(d, vg, conf.Meter, time.Duration(poll)*time.Second)
	}
	return nil
}

// meterReader is a loop that reads the data from the energy meter.
func meterReader(d *db.DB, vg string, url string, delay time.Duration) {
	lastTime := time.Now()
	client := http.Client{
		Timeout: time.Duration(time.Second * 10),
	}
	for {
		time.Sleep(delay - time.Now().Sub(lastTime))
		lastTime = time.Now()
		err := fetch(d, vg, &client, url)
		if err != nil {
			log.Printf("iammeter: %v", err)
		}
	}
}

func fetch(d *db.DB, vg string, client *http.Client, url string) error {
	type Top struct {
		Method  string    `json:"method"`
		Mac     string    `json:"mac"`
		Version string    `json:"version"`
		Server  string    `json:"server"`
		Serial  string    `json:"SN"`
		Data    []float64 `json:"Data"`
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var m Top
	err = json.Unmarshal(body, &m)
	if err != nil {
		return err
	}
	if len(m.Data) != 5 {
		return fmt.Errorf("malformed data from meter")
	}
	if d.Trace {
		log.Printf("iammeter: version %s, serial number %s", m.Version, m.Serial)
		log.Printf("iammeter: Volts %gV, current %gA, power %gW", m.Data[0], m.Data[1], m.Data[2])
		log.Printf("iammeter: Import %gkWh, Export %gkWh", m.Data[3], m.Data[4])
	}
	if len(m.Version) == 0 || len(m.Serial) == 0 || m.Data[0] == 0 ||
		m.Data[3] == 0 || m.Data[4] == 0 {
		return fmt.Errorf("Missing values")
	}
	d.Input(vg, m.Data[0])
	if m.Data[1] < 0.0 {
		d.Input(db.G_IN_CURRENT, 0.0)
		d.Input(db.G_OUT_CURRENT, -m.Data[1])
	} else {
		d.Input(db.G_OUT_CURRENT, 0.0)
		d.Input(db.G_IN_CURRENT, m.Data[1])
	}
	// ImportEnergy
	d.Input(db.D_IN_POWER, m.Data[3])
	d.Input(db.A_IN_TOTAL, m.Data[3])
	d.Input(db.A_IMPORT, m.Data[3])
	// ExportGrid
	d.Input(db.D_OUT_POWER, m.Data[4])
	d.Input(db.A_OUT_TOTAL, m.Data[4])
	d.Input(db.A_EXPORT, m.Data[4])
	return nil
}
