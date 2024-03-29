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

// package sma implements reading telemetry data from a SMA solar inverter.
// The package is configured as a section in the main config file
// under one or more '[sma]' sections, and the parameters are:
//   [sma]
//   inverter=<inverter-name>:<udp-port>,<password>[,poll-time-seconds]

package sma

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var smaPoll = flag.Int("inverter-poll", 90, "Default inverter poll time (seconds)")
var smaRetry = flag.Int("inverter-retry", 15, "Inverter poll retry time (seconds)")
var smaVolts = flag.Bool("inverter-volts", false, "Send inverter Volts reading")

// InverterReader polls the inverter(s)
type InverterReader struct {
	d   *db.DB // Database
	sma *SMA   // Inverter object
	// Database element names. These are dynamically allocated.
	genP     string // Gauge for current power (Kw)
	genDP    string // Derived for daily yield (KwH)
	volts    string // Gauge for current voltage (V)
	genDaily string // Accum for daily yield (KwH)
	genT     string // Accum for lifetime yield (KwH)
}

func init() {
	db.RegisterInit(inverterReader)
}

// Initialise SMA reader(s).
func inverterReader(d *db.DB) error {
	for _, sect := range d.Config.GetSections("sma") {
		for _, e := range sect.Get("inverter") {
			poll := *smaPoll
			// Inverter config is of the form [IP address|name]:port,password[,poll]
			if len(e.Tokens) == 3 {
				if v, err := strconv.ParseInt(e.Tokens[2], 10, 32); err != nil {
					return fmt.Errorf("%s:%d: Inverter poll value error: %v", e.Filename, e.Lineno, err)
				} else {
					poll = int(v)
				}
			} else if len(e.Tokens) != 2 {
				return fmt.Errorf("%s:%d: Inverter config error", e.Filename, e.Lineno)
			}
			sma, err := NewSMA(e.Tokens[0], e.Tokens[1])
			if err != nil {
				return err
			}
			s := &InverterReader{d: d, sma: sma}
			// Allocate gauges etc. for the inverter.
			s.genP = d.AddSubGauge(db.G_GEN_P, false)
			if *smaVolts {
				s.volts = d.AddSubGauge(db.G_VOLTS, true)
			}
			s.genDaily = d.AddSubAccum(db.A_GEN_DAILY, true)
			s.genT = d.AddSubAccum(db.A_GEN_TOTAL, false)
			s.genDP = d.AddSubDiff(db.D_GEN_P, false)
			log.Printf("Registered SMA inverter reader for %s (polling interval %d seconds)\n", s.sma.Name(), poll)
			go s.run(time.Duration(poll)*time.Second, time.Duration(*smaRetry)*time.Second)
		}
	}
	return nil
}

// Polling loop for inverter.
func (s *InverterReader) run(poll, retry time.Duration) {
	defer s.sma.Close()
	for {
		hour := time.Now().Hour()
		err := s.poll(hour >= s.d.StartHour && hour < s.d.EndHour)
		if err != nil {
			log.Printf("Inverter poll error:%s - %v", s.sma.Name(), err)
			time.Sleep(retry)
		} else {
			time.Sleep(poll)
		}
	}
}

func (s *InverterReader) poll(daytime bool) error {
	if s.d.Trace {
		log.Printf("Polling inverter %s", s.sma.Name())
	}
	_, _, err := s.sma.Logon()
	if err != nil {
		return err
	}
	defer s.sma.Logoff()
	d, err := s.sma.DailyEnergy()
	if err != nil {
		if s.d.Trace {
			log.Printf("Missing record for tag %s", s.genDaily)
		}
	} else {
		if s.d.Trace {
			log.Printf("Tag %s Daily yield = %f", s.genDaily, d)
		}
		s.d.Input(s.genDaily, d)
	}
	t, err := s.sma.TotalEnergy()
	if err != nil {
		if s.d.Trace {
			log.Printf("Missing record for tag %s", s.genT)
		}
	} else {
		if s.d.Trace {
			log.Printf("Tag %s Total yield = %f", s.genT, t)
		}
		s.d.Input(s.genT, t)
		s.d.Input(s.genDP, t)
	}
	if daytime {
		if *smaVolts {
			v, err := s.sma.Voltage()
			if err != nil {
				return err
			}
			if v != 0 {
				if s.d.Trace {
					log.Printf("Tag %s volts = %f", s.volts, v)
				}
				s.d.Input(s.volts, v)
			}
		}
		p, err := s.sma.Power()
		if err != nil {
			return err
		}
		pf := float64(p) / 1000
		if s.d.Trace {
			log.Printf("Tag %s power = %f", s.genP, pf)
		}
		s.d.Input(s.genP, pf)
	}
	return nil
}
