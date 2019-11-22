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

package sma

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aamcrae/MeterMan/core"
	"github.com/aamcrae/config"
)

var smaPoll = flag.Int("inverter-poll", 120, "Inverter poll time (seconds)")
var smaRetry = flag.Int("inverter-retry", 10, "Inverter poll retry time (seconds)")

// InverterReader polls the inverter(s)
type InverterReader struct {
	sma      *SMA
	genP     string
	volts    string
	genDaily string
	genT     string
}

func init() {
	core.RegisterReader(inverterReader)
}

func inverterReader(conf *config.Config, wr chan<- core.Input) error {
	sect := conf.GetSection("sma")
	if sect == nil {
		return nil
	}
	log.Printf("Registered SMA inverter reader\n")
	// Inverter name is of the format [IP address|name]:port,password
	for _, e := range sect.Get("inverter") {
		if len(e.Tokens) != 2 {
			return fmt.Errorf("Inverter config error at line %d", e.Lineno)
		}
		sma, err := NewSMA(e.Tokens[0], e.Tokens[1])
		if err != nil {
			return err
		}
		s := &InverterReader{sma: sma}
		s.genP = core.AddSubGauge(core.G_GEN_P, false)
		s.volts = core.AddSubGauge(core.G_VOLTS, true)
		s.genDaily = core.AddSubAccum(core.A_GEN_DAILY, true)
		s.genT = core.AddSubAccum(core.A_GEN_TOTAL, false)
		go s.run(wr)
	}
	return nil
}

func (s *InverterReader) run(wr chan<- core.Input) {
	defer s.sma.Close()
	for {
		hour := time.Now().Hour()
		err := s.poll(wr, hour >= *core.StartHour && hour < *core.EndHour)
		if err != nil {
			log.Printf("Inverter poll error:%s - %v", s.sma.Name(), err)
			time.Sleep(time.Duration(*smaRetry) * time.Second)
		} else {
			time.Sleep(time.Duration(*smaPoll) * time.Second)
		}
	}
}

func (s *InverterReader) poll(wr chan<- core.Input, daytime bool) error {
	if *core.Verbose {
		log.Printf("Polling inverter %s", s.sma.Name())
	}
	_, _, err := s.sma.Logon()
	if err != nil {
		return err
	}
	defer s.sma.Logoff()
	d, err := s.sma.DailyEnergy()
	if err != nil {
		return err
	}
	t, err := s.sma.TotalEnergy()
	if err != nil {
		return err
	}
	if *core.Verbose {
		log.Printf("Tag %s Daily yield = %f, tag %s total yield = %f", s.genDaily, d, s.genT, t)
	}
	wr <- core.Input{Tag: s.genDaily, Value: d}
	wr <- core.Input{Tag: s.genT, Value: t}
	if daytime {
		v, err := s.sma.Voltage()
		if err != nil {
			return err
		}
		if v != 0 {
			if *core.Verbose {
				log.Printf("Tag %s volts = %f", s.volts, v)
			}
			wr <- core.Input{Tag: s.volts, Value: v}
		}
		p, err := s.sma.Power()
		if err != nil {
			return err
		}
		if p != 0 {
			pf := float64(p) / 1000
			if *core.Verbose {
				log.Printf("Tag %s power = %f", s.genP, pf)
			}
			wr <- core.Input{Tag: s.genP, Value: pf}
		}
	}
	return nil
}
