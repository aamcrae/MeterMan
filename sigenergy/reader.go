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
	"sync/atomic"
	"time"

	"github.com/aamcrae/MeterMan/core"
)

const retries = 3

type Sigenergy struct {
	Addr    string
	Unit    int
	Size    float64
	Timeout int
	Trace   bool
}

// SigenergyReader polls the battery
type SigenergyReader struct {
	d      *core.DB       // Database
	size   float64      // Size of battery in kWh
	batt   *Battery     // Battery object
	status atomic.Value // Current status
}

func init() {
	core.RegisterInit(batteryReader)
}

// Initialise Sigenergy reader(s).
func batteryReader(d *core.DB) error {
	var conf Sigenergy
	c, ok := d.Config["sigenergy"]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	unit := uint8(core.ConfigOrDefault(conf.Unit, 247)) // Default poll interval of 60 seconds
	size := core.ConfigOrDefault(conf.Size, 32.23)      // Default size of battery is around 32kWh
	batt, err := NewBattery(conf.Addr, unit)
	if err != nil {
		return err
	}
	batt.Timeout = core.ConfigOrDefault(time.Second*time.Duration(conf.Timeout), batt.Timeout)
	batt.Trace = conf.Trace
	s := &SigenergyReader{d: d, size: size, batt: batt}
	s.status.Store("init")
	d.AddStatusPrinter("Battery", s.Status)
	log.Printf("Registered SigEnergy battery reader for %s (timeout %s)\n", conf.Addr, s.batt.Timeout.String())
	if !d.Dryrun {
		d.AddGauge(core.G_BATT_POWER)
		d.AddGauge(core.G_BATT_SIZE)
		d.AddGauge(core.G_BATT_PERCENT)
		d.AddGauge(core.G_BATT_STATUS)
		d.AddAccum(core.A_CHARGE_TOTAL, false)
		d.AddAccum(core.A_DISCHARGE_TOTAL, false)
		d.AddPoll(s.cbPoll)
	}
	return nil
}

// Status returns a string status for this inverter
func (s *SigenergyReader) Status() string {
	return s.status.Load().(string)
}

func (s *SigenergyReader) cbPoll() {
	var err error
	for _ = range retries {
		err = s.poll()
		if err == nil {
			return
		}
	}
	log.Printf("Battery poll error: %v", err)
}

func (s *SigenergyReader) poll() error {
	if s.d.Trace {
		log.Printf("Polling battery")
	}
	var b strings.Builder
	defer func() { s.status.Store(b.String()) }()
	fmt.Fprintf(&b, "%s: ", time.Now().Format("2006-01-02 15:04"))
	err := s.batt.poll()
	if err != nil {
		fmt.Fprintf(&b, "Error - %v", err)
		return err
	}
	s.d.Input(core.G_BATT_POWER, s.batt.power)
	s.d.Input(core.G_BATT_PERCENT, s.batt.percent)
	s.d.Input(core.G_BATT_SIZE, s.size)
	s.d.Input(core.G_BATT_STATUS, float64(core.BATT_ENABLED))
	s.d.Input(core.A_CHARGE_TOTAL, s.batt.acc_charge)
	s.d.Input(core.A_DISCHARGE_TOTAL, s.batt.acc_discharge)
	fmt.Fprintf(&b, "OK")
	fmt.Fprintf(&b, ", Grid Power %s", core.FmtFloat(s.batt.grid_power))
	fmt.Fprintf(&b, ", Battery percent %s", core.FmtFloat(s.batt.percent))
	fmt.Fprintf(&b, ", Battery power %s", core.FmtFloat(s.batt.power))
	fmt.Fprintf(&b, ", Accum charge %s", core.FmtFloat(s.batt.acc_charge))
	fmt.Fprintf(&b, ", Accum discharge %s", core.FmtFloat(s.batt.acc_discharge))
	return nil
}
