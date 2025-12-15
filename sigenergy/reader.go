// Copyright 2025 Andrew McRae
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

// package sigenergy implements reading telemetry data from a SigEnergy battery.

package sigenergy

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lib"
)

type Sigenergy struct {
	Addr    string
	Unit    int
	Size    float64
	Poll    int
	Offset  int
	Timeout int
	Trace   bool
}

// SigenergyReader polls the battery
type SigenergyReader struct {
	d      *db.DB   // Database
	size   float64  // Size of battery in kWh
	batt   *Battery // Battery object
	status string   // Current status
}

func init() {
	db.RegisterInit(batteryReader)
}

// Initialise Sigenergy reader(s).
func batteryReader(d *db.DB) error {
	var conf Sigenergy
	c, ok := d.Config["sigenergy"]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	unit := uint8(lib.ConfigOrDefault(conf.Unit, 247)) // Default poll interval of 60 seconds
	size := lib.ConfigOrDefault(conf.Size, 32.23)      // Default size of battery is around 32kWh
	poll := lib.ConfigOrDefault(conf.Poll, 60)         // Default poll interval of 60 seconds
	offset := lib.ConfigOrDefault(conf.Offset, -5)     // Default offset of -5 seconds
	batt, err := NewBattery(conf.Addr, unit)
	if err != nil {
		return err
	}
	batt.Timeout = lib.ConfigOrDefault(time.Second*time.Duration(conf.Timeout), batt.Timeout)
	batt.Trace = conf.Trace
	s := &SigenergyReader{d: d, size: size, batt: batt}
	d.AddStatusPrinter("Battery", s.Status)
	log.Printf("Registered SigEnergy battery reader for %s (poll interval %d seconds, offset %d seconds, timeout %s)\n", conf.Addr, poll, offset, s.batt.Timeout.String())
	if !d.Dryrun {
		s.d.AddGauge(db.G_BATT_POWER)
		s.d.AddGauge(db.G_BATT_SIZE)
		s.d.AddGauge(db.G_BATT_PERCENT)
		s.d.AddGauge(db.G_BATT_STATUS)
		s.d.AddAccum(db.A_CHARGE_TOTAL, false)
		s.d.AddAccum(db.A_DISCHARGE_TOTAL, false)
		d.AddCallback(time.Second*time.Duration(poll), time.Second*time.Duration(offset), func(now time.Time) {
			go s.cbPoll(now)
		})
	}
	return nil
}

// Status returns a string status for this inverter
func (s *SigenergyReader) Status() string {
	return s.status
}

func (s *SigenergyReader) cbPoll(now time.Time) {
	err := s.poll()
	if err != nil {
		log.Printf("Battery poll error: %v", err)
	}
}

func (s *SigenergyReader) poll() error {
	if s.d.Trace {
		log.Printf("Polling battery")
	}
	var b strings.Builder
	defer func() { s.status = b.String() }()
	fmt.Fprintf(&b, "%s: ", time.Now().Format("2006-01-02 15:04"))
	err := s.batt.poll()
	if err != nil {
		fmt.Fprintf(&b, "Error - %v", err)
		return err
	}
	s.d.Input(db.G_BATT_POWER, s.batt.power)
	s.d.Input(db.G_BATT_PERCENT, s.batt.percent)
	s.d.Input(db.G_BATT_SIZE, s.size)
	s.d.Input(db.G_BATT_STATUS, float64(db.BATT_ENABLED))
	s.d.Input(db.A_CHARGE_TOTAL, s.batt.acc_charge)
	s.d.Input(db.A_DISCHARGE_TOTAL, s.batt.acc_discharge)
	fmt.Fprintf(&b, "OK")
	fmt.Fprintf(&b, ", Grid Power %s", lib.FmtFloat(s.batt.grid_power))
	fmt.Fprintf(&b, ", Battery percent %s", lib.FmtFloat(s.batt.percent))
	fmt.Fprintf(&b, ", Battery power %s", lib.FmtFloat(s.batt.power))
	fmt.Fprintf(&b, ", Accum charge %s", lib.FmtFloat(s.batt.acc_charge))
	fmt.Fprintf(&b, ", Accum discharge %s", lib.FmtFloat(s.batt.acc_discharge))
	return nil
}
