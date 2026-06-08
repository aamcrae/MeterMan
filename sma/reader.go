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
// The package is configured as a section in the YAML config file:
//   sma:
//     - addr: <inverter-name:udp-port>
//       password: <password>
//       retry: <poll-retry-seconds>
//     - ...

package sma

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aamcrae/MeterMan/core"
)

const retries = 3

type Sma []struct {
	Addr     string
	Password string
	Timeout  int
	Volts    bool
	Trace    bool
	Dump     bool
}

// InverterReader polls the inverter(s)
type InverterReader struct {
	d   *core.DB // Database
	sma *SMA   // Inverter object
	// Database element names. These are dynamically allocated.
	genP     string       // Gauge for current power (Kw)
	genDP    string       // Derived for daily yield (KwH)
	volts    string       // Gauge for current voltage (V)
	genDaily string       // Accum for daily yield (KwH)
	genT     string       // Accum for lifetime yield (KwH)
	mpttA    string       // MPTT A string
	mpttB    string       // MPTT B string
	status   atomic.Value // Current status
}

func init() {
	core.RegisterInit(inverterReader)
}

// Initialise SMA reader(s).
func inverterReader(d *core.DB) error {
	var conf Sma
	c, ok := d.Config["sma"]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	for index, e := range conf {
		sma, err := NewSMA(e.Addr, e.Password)
		if err != nil {
			return err
		}
		sma.Timeout = core.ConfigOrDefault(time.Second*time.Duration(e.Timeout), sma.Timeout)
		sma.Trace = e.Trace
		sma.PktDump = e.Dump
		s := &InverterReader{d: d, sma: sma}
		// Allocate gauges etc. for the inverter.
		s.genP = d.AddSubGauge(core.G_GEN_P, false)
		if e.Volts {
			s.volts = d.AddSubGauge(core.G_VOLTS, true)
		}
		s.genDaily = d.AddSubAccum(core.A_GEN_DAILY, true)
		s.genT = d.AddSubAccum(core.A_GEN_TOTAL, false)
		s.genDP = d.AddSubDiff(core.D_GEN_P, false)
		mptt := fmt.Sprintf("%s-%d", core.G_MPTT, index)
		s.mpttA = fmt.Sprintf("%s-A", mptt)
		s.mpttB = fmt.Sprintf("%s-B", mptt)
		s.status.Store("init")
		d.AddGauge(s.mpttA)
		d.AddGauge(s.mpttB)
		nm := strings.Split(e.Addr, ":")[0]
		d.AddStatusPrinter(fmt.Sprintf("SMA-%s", nm), s.Status)
		log.Printf("Registered SMA inverter reader for %s (timeout %s)\n", s.sma.Name(), s.sma.Timeout.String())
		if !d.Dryrun {
			d.AddPoll(s.cbPoll)
		}
	}
	return nil
}

// Status returns a string status for this inverter
func (s *InverterReader) Status() string {
	return s.status.Load().(string)
}

func (s *InverterReader) cbPoll() {
	hour := time.Now().Hour()
	daytime := hour >= s.d.StartHour && hour < s.d.EndHour
	var err error
	for _ = range retries {
		err = s.poll(daytime)
		if err == nil {
			return
		}
		time.Sleep(time.Second * 5)
	}
	log.Printf("Inverter poll error:%s - %v", s.sma.Name(), err)
}

func (s *InverterReader) poll(daytime bool) error {
	if s.d.Trace {
		log.Printf("Polling inverter %s", s.sma.Name())
	}
	var b strings.Builder
	defer func() { s.status.Store(b.String()) }()
	fmt.Fprintf(&b, "%s: ", time.Now().Format("2006-01-02 15:04"))
	_, _, err := s.sma.Logon()
	if err != nil {
		fmt.Fprintf(&b, "Error - %v", err)
		return err
	}
	defer s.sma.Logoff()
	fmt.Fprintf(&b, "OK")
	d, err := s.sma.DailyEnergy()
	if err != nil {
		if s.d.Trace {
			log.Printf("Missing record for tag %s", s.genDaily)
		}
	} else {
		if s.d.Trace {
			log.Printf("Tag %s Daily yield = %g", s.genDaily, d)
		}
		s.d.Input(s.genDaily, d)
		fmt.Fprintf(&b, ", Daily %s", core.FmtFloat(d))
	}
	t, err := s.sma.TotalEnergy()
	if err != nil {
		if s.d.Trace {
			log.Printf("Missing record for tag %s", s.genT)
		}
	} else {
		if s.d.Trace {
			log.Printf("Tag %s Total yield = %g", s.genT, t)
		}
		fmt.Fprintf(&b, ", Total %s", core.FmtFloat(t))
		s.d.Input(s.genT, t)
		s.d.Input(s.genDP, t)
	}
	if !daytime {
		return nil // Some values are not available at night
	}
	if len(s.volts) != 0 {
		v, err := s.sma.Voltage()
		if err != nil {
			return err
		}
		if v != 0 {
			if s.d.Trace {
				log.Printf("Tag %s volts = %g", s.volts, v)
			}
			s.d.Input(s.volts, v)
			fmt.Fprintf(&b, ", Volts %s", core.FmtFloat(v))
		}
	}
	p, err := s.sma.Power()
	if err != nil {
		return err
	}
	pf := float64(p) / 1000
	if s.d.Trace {
		log.Printf("Tag %s power = %g", s.genP, pf)
	}
	s.d.Input(s.genP, pf)
	fmt.Fprintf(&b, ", Power %s", core.FmtFloat(pf))

	mptts, err := s.sma.MPTT()
	if err != nil {
		return err
	}
	if len(mptts) != 2 {
		log.Printf("sma:%s: wrong len of mptt: %d - ignored", s.sma.Name(), len(mptts))
	} else {
		if s.d.Trace {
			log.Printf("Tag %s = %g, %s = %g", s.mpttA, mptts[0], s.mpttB, mptts[1])
		}
		s.d.Input(s.mpttA, mptts[0])
		s.d.Input(s.mpttB, mptts[1])
		fmt.Fprintf(&b, ", MPPT-A %s, MPTT-B %s", core.FmtFloat(mptts[0]), core.FmtFloat(mptts[1]))
	}
	return nil
}
