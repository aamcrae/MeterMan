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

// package pv implements a writer that uploads the current data to pvoutput.org.
// The URL parameters that are uploaded are:
// d  - Date in YYYYMMDD format
// t  - Time in HH:MM format
// v1 - PV daily generation (energy) (wH)
// v2 - PV current power output (w)
// v3 - Daily energy consumption (wH)
// v4 - Current power consumption (w)
// v5 - Temperature (C)
// v6 - AC voltage (V)
//
// The package is configured as a section in the YAML config file:
//  pvoutput:
//    apikey: <apikey from pvoutput.org>
//    systemid: <systemid from pvoutput.org>
//    pvurl: <URL API endpoint to use>

package pv

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lib"
)

type Pvoutput struct {
	Apikey   string
	Systemid string
	Pvurl    string
	Interval int
	Trace    bool
}

const defaultInterval = 5 // Default update rate in minutes

const moduleName = "pvoutput"

type pvWriter struct {
	d      *db.DB
	pvurl  string
	id     string
	key    string
	client *http.Client
	trace  bool
	status string
}

func init() {
	db.RegisterInit(pvoutputInit)
}

func pvoutputInit(d *db.DB) error {
	var conf Pvoutput
	c, ok := d.Config[moduleName]
	if !ok {
		return nil
	}
	err := c.Decode(&conf)
	if err != nil {
		return err
	}
	interval := lib.ConfigOrDefault(conf.Interval, defaultInterval)
	url := lib.ConfigOrDefault(conf.Pvurl, "https://pvoutput.org/service/r2/addstatus.jsp")
	p := &pvWriter{d: d, pvurl: url, id: conf.Systemid, key: conf.Apikey, client: &http.Client{}, trace: conf.Trace || d.Trace, status: "init"}
	if !d.Dryrun {
		d.AddCallback(time.Minute*time.Duration(interval), p.upload)
	}
	d.AddStatusPrinter(moduleName, p.Status)
	log.Printf("Registered pvoutput uploader (%d minute intervals)\n", interval)
	return nil
}

// Run creates a post request to pvoutput.org to upload the current data.
func (p *pvWriter) upload(now time.Time) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: ", now.Format("2006-01-02 15:04"))
	pv_power, pv_power_ok := p.getPVPower()
	pv_daily, pv_daily_ok := p.getPVDaily()
	temp := p.d.GetElement(db.G_TEMP)
	volts := p.d.GetElement(db.G_VOLTS)
	imp := p.d.GetAccum(db.A_IN_TOTAL)
	exp := p.d.GetAccum(db.A_OUT_TOTAL)
	hour := now.Hour()
	daytime := hour >= p.d.StartHour && hour < p.d.EndHour

	val := url.Values{}
	val.Add("d", now.Format("20060102"))
	val.Add("t", now.Format("15:04"))
	if pv_daily_ok && daytime {
		val.Add("v1", fmt.Sprintf("%d", int(pv_daily*1000)))
		if p.trace {
			log.Printf("v1 = %g", pv_daily)
		}
	} else if p.trace {
		if !pv_daily_ok {
			log.Printf("PV Energy not valid, v1 not updated\n")
		}
	}
	if pv_power_ok && pv_power != 0 {
		val.Add("v2", fmt.Sprintf("%d", int(pv_power*1000)))
		if p.trace {
			log.Printf("v2 = %g", pv_power)
		}
	} else if p.trace {
		log.Printf("No PV power, v2 not updated\n")
	}
	if isValid(temp) && temp.Get() != 0 {
		val.Add("v5", fmt.Sprintf("%.2f", temp.Get()))
		if p.trace {
			log.Printf("v5 = %.2f", temp.Get())
		}
	} else if p.trace {
		log.Printf("No temperature, v5 not updated\n")
	}
	if isValid(volts) && volts.Get() != 0 {
		val.Add("v6", fmt.Sprintf("%.2f", volts.Get()))
		if p.trace {
			log.Printf("v6 = %.2f", volts.Get())
		}
	} else if p.trace {
		log.Printf("No Voltage, v6 not updated\n")
	}
	if isValid(imp) && isValid(exp) {
		consumption := imp.Daily() - exp.Daily()
		// Daily PV generation may be out of date, but it is used regardless.
		consumption += pv_daily
		val.Add("v3", fmt.Sprintf("%d", int(consumption*1000)))
		if p.trace {
			log.Printf("v3 = %g, imp = %g, exp = %g", consumption, imp.Daily(), exp.Daily())
			log.Printf("daily = %g", pv_daily)
			if !pv_daily_ok {
				log.Printf("Using old generation data")
			}
		}
	} else if p.trace {
		if exp == nil {
			log.Printf("No export data\n")
		} else if !isValid(exp) {
			log.Printf("Export data not fresh\n")
		}
		if imp == nil {
			log.Printf("No import data\n")
		} else if !isValid(imp) {
			log.Printf("Import data not fresh\n")
		}
		log.Printf("No consumption data, v3 not updated\n")
	}
	tp, err := p.getPower()
	if err == nil {
		var g float64
		if pv_power_ok {
			g = pv_power
		}
		cp := int(g*1000 + tp)
		if cp < 0 {
			log.Printf("Negative power consumption (%d), v4 set to 0, gen = %d, meter = %d\n", cp, int(g*1000), int(tp))
			cp = 0
		}
		val.Add("v4", fmt.Sprintf("%d", cp))
		if p.trace {
			log.Printf("v4 = %d", cp)
		}
	} else {
		log.Printf("Invalid total power, v4 not sent: %v\n", err)
	}
	req, err := http.NewRequest("POST", p.pvurl, strings.NewReader(val.Encode()))
	if err != nil {
		log.Printf("NewRequest failed: %v", err)
		fmt.Fprintf(&b, "NewRequest err: %v", err)
		p.status = b.String()
		return
	}
	if p.trace {
		log.Printf("PV Uploading: %v", val)
	}
	fmt.Fprintf(&b, "Upload: %v", val)
	req.Header.Add("X-Pvoutput-Apikey", p.key)
	req.Header.Add("X-Pvoutput-SystemId", p.id)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.trace {
		log.Printf("PV req: %s (size %d)", val.Encode(), req.ContentLength)
	}
	// Asynchronously send request to avoid blocking.
	go p.send(req, &b)
}

func (p *pvWriter) Status() string {
	return p.status
}

// Send request to server.
func (p *pvWriter) send(req *http.Request, b *strings.Builder) {
	defer func() { p.status = b.String() }()
	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf(" - Request failed: %v", err)
		fmt.Fprintf(b, "Req err: %v", err)
		return
	}
	defer resp.Body.Close()
	if p.trace {
		log.Printf("Response is: %s", resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error: %s: %s", resp.Status, body)
		fmt.Fprintf(b, "Resp err: %s", resp.Status)
	} else {
		fmt.Fprintf(b, " - OK")
	}
}

// getPVPower returns the current PV power.
// If it is not valid, an attempt is made to derive it from any
// valid sub-values.
func (p *pvWriter) getPVPower() (float64, bool) {
	pwr := p.d.GetElement(db.D_GEN_P)
	if isValid(pwr) {
		return pwr.Get(), true
	}
	if p.trace {
		log.Printf("%s not valid, trying sub-values", db.A_GEN_TOTAL)
	}
	for i := 0; i < 2; i++ {
		pe := p.d.GetElement(fmt.Sprintf("%s/%d", db.D_GEN_P, i))
		if pe == nil {
			break
		}
		if isValid(pe) {
			if p.trace {
				log.Printf("Using 2 x %s/%d (value %g)", db.D_GEN_P, i, pe.Get())
			}
			return pe.Get() * 2, true
		}
	}
	if pwr != nil {
		return pwr.Get(), false
	}
	return 0, false
}

// getPVDaily returns the PV daily generation.
// If it is not valid, an attempt is made to derive it from any
// valid sub-values.
func (p *pvWriter) getPVDaily() (float64, bool) {
	pd := p.d.GetAccum(db.A_GEN_TOTAL)
	if isValid(pd) {
		return pd.Daily(), true
	}
	if p.trace {
		log.Printf("%s not valid, trying sub-values", db.A_GEN_TOTAL)
	}
	for i := 0; i < 2; i++ {
		pe := p.d.GetAccum(fmt.Sprintf("%s/%d", db.A_GEN_TOTAL, i))
		if pe == nil {
			break
		}
		if isValid(pe) {
			if p.trace {
				log.Printf("Using 2 x %s/%d (value %g)", db.A_GEN_TOTAL, i, pe.Daily())
			}
			return pe.Daily() * 2, true
		}
	}
	if pd != nil {
		return pd.Daily(), false
	}
	return 0, false
}

// getPower returns the current import/export power (as Watts)
func (p *pvWriter) getPower() (float64, error) {
	d_in := p.d.GetElement(db.D_IN_POWER)
	d_out := p.d.GetElement(db.D_OUT_POWER)
	if p.trace {
		log.Printf("IN-P  = %g, valid = %v", d_in.Get(), isValid(d_in))
		log.Printf("OUT-P = %g, valid = %v", d_out.Get(), isValid(d_out))
	}
	if isValid(d_in) && isValid(d_out) {
		return (d_in.Get() - d_out.Get()) * 1000.0, nil
	}
	return 0.0, fmt.Errorf("no valid power reading")
}

// isValid will return true if the element is not nil and is fresh
func isValid(e db.Element) bool {
	return e != nil && e.Fresh()
}
