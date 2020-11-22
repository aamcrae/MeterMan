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
// under the '[sma]' section, and the parameters are:
//   [sma]
//   inverter=<inverter-name>:<udp-port>,<password>

package sma

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var smaPoll = flag.Int("inverter-poll", 90, "Inverter poll time (seconds)")
var smaRetry = flag.Int("inverter-retry", 10, "Inverter poll retry time (seconds)")

// InverterReader polls the inverter(s)
type InverterReader struct {
	d        *db.DB
	sma      *SMA
	genP     string
	genDP    string
	volts    string
	genDaily string
	genT     string
}

func init() {
	db.RegisterInit(inverterReader)
}

func inverterReader(d *db.DB) error {
	sl := d.Config.GetSections("sma")
	if len(sl) == 0 {
		return nil
	}
	for _, sect := range sl {
		// Inverter name is of the format [IP address|name]:port,password
		le := sect.Get("inverter")
		if len(le) != 1 {
			return fmt.Errorf("Missing or duplicate inverter configuration")
		}
		e := le[0]
		if len(e.Tokens) != 2 {
			return fmt.Errorf("Inverter config error at line %d", e.Lineno)
		}
		sma, err := NewSMA(e.Tokens[0], e.Tokens[1])
		if err != nil {
			return err
		}
		s := &InverterReader{d: d, sma: sma}
		s.genP = d.AddSubGauge(db.G_GEN_P, false)
		s.volts = d.AddSubGauge(db.G_VOLTS, true)
		s.genDaily = d.AddSubAccum(db.A_GEN_DAILY, true)
		s.genT = d.AddSubAccum(db.A_GEN_TOTAL, false)
		s.genDP = d.AddSubDiff(db.D_GEN_P, false)
		log.Printf("Registered SMA inverter reader for %s\n", s.sma.Name())
		go s.run()
	}
	return nil
}

func (s *InverterReader) run() {
	defer s.sma.Close()
	for {
		hour := time.Now().Hour()
		err := s.poll(hour >= s.d.StartHour && hour < s.d.EndHour)
		if err != nil {
			log.Printf("Inverter poll error:%s - %v", s.sma.Name(), err)
			time.Sleep(time.Duration(*smaRetry) * time.Second)
		} else {
			time.Sleep(time.Duration(*smaPoll) * time.Second)
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
		s.d.InChan <- db.Input{Tag: s.genDaily, Value: d}
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
		s.d.InChan <- db.Input{Tag: s.genT, Value: t}
		s.d.InChan <- db.Input{Tag: s.genDP, Value: t}
	}
	if daytime {
		v, err := s.sma.Voltage()
		if err != nil {
			return err
		}
		if v != 0 {
			if s.d.Trace {
				log.Printf("Tag %s volts = %f", s.volts, v)
			}
			s.d.InChan <- db.Input{Tag: s.volts, Value: v}
		}
		p, err := s.sma.Power()
		if err != nil {
			return err
		}
		pf := float64(p) / 1000
		if s.d.Trace {
			log.Printf("Tag %s power = %f", s.genP, pf)
		}
		s.d.InChan <- db.Input{Tag: s.genP, Value: pf}
	}
	return nil
}
