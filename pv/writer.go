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
// The package is configured as a section in the main config file
// under the '[pvoutput]' section, and the parameters are:
//  [pvoutput]
//  apikey=<apikey from pvoutput.org>
//  systemid=<systemid from pvoutput.org>
//  pvurl=<URL API endpoint to use>

package pv

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var dryrun = flag.Bool("dryrun", false, "Do not upload data")
var pvLog = flag.Bool("pvlog", true, "Log upload parameters")
var pvUpdateRate = flag.Int("pvupdate", 5, "pvoutput Update rate (in minutes)")

type pvWriter struct {
	d      *db.DB
	pvurl  string
	id     string
	key    string
	client *http.Client
}

func init() {
	db.RegisterInit(pvoutputInit)
}

func pvoutputInit(d *db.DB) error {
	sect := d.Config.GetSection("pvoutput")
	if sect == nil {
		return nil
	}
	key, err := sect.GetArg("apikey")
	if err != nil {
		return err
	}
	id, err := sect.GetArg("systemid")
	if err != nil {
		return err
	}
	pvurl, err := sect.GetArg("pvurl")
	if err != nil {
		return err
	}
	p := &pvWriter{d: d, pvurl: pvurl, id: id, key: key, client: &http.Client{}}
	d.AddUpdate(p, time.Minute*time.Duration(*pvUpdateRate))
	log.Printf("Registered pvoutput uploader\n")
	return nil
}

// pvUpload creates a post request to pvoutput.org to upload the current data.
func (p *pvWriter) Update(last time.Time, now time.Time) {
	pv_power := p.d.GetElement(db.G_GEN_P)
	temp := p.d.GetElement(db.G_TEMP)
	volts := p.d.GetElement(db.G_VOLTS)
	pv_daily := p.d.GetAccum(db.A_GEN_TOTAL)
	imp := p.d.GetAccum(db.A_IN_TOTAL)
	exp := p.d.GetAccum(db.A_OUT_TOTAL)
	hour := now.Hour()
	daytime := hour >= p.d.StartHour && hour < p.d.EndHour

	val := url.Values{}
	val.Add("d", now.Format("20060102"))
	val.Add("t", now.Format("15:04"))
	if isValid(pv_daily, last) && daytime {
		val.Add("v1", fmt.Sprintf("%d", int(pv_daily.Daily()*1000)))
		if p.d.Trace {
			log.Printf("v1 = %f", pv_daily.Daily())
		}
	} else if p.d.Trace {
		if pv_daily == nil {
			log.Printf("No PV energy total, v1 not updated\n")
		} else {
			log.Printf("PV Energy not fresh, v1 not updated\n")
		}
	}
	if isValid(pv_power, last) && pv_power.Get() != 0 {
		val.Add("v2", fmt.Sprintf("%d", int(pv_power.Get()*1000)))
		if p.d.Trace {
			log.Printf("v2 = %f", pv_power.Get())
		}
	} else if p.d.Trace {
		log.Printf("No PV power, v2 not updated\n")
	}
	if isValid(temp, last) && temp.Get() != 0 {
		val.Add("v5", fmt.Sprintf("%.2f", temp.Get()))
		if p.d.Trace {
			log.Printf("v5 = %.2f", temp.Get())
		}
	} else if p.d.Trace {
		log.Printf("No temperature, v5 not updated\n")
	}
	if isValid(volts, last) && volts.Get() != 0 {
		val.Add("v6", fmt.Sprintf("%.2f", volts.Get()))
		if p.d.Trace {
			log.Printf("v6 = %.2f", volts.Get())
		}
	} else if p.d.Trace {
		log.Printf("No Voltage, v6 not updated\n")
	}
	if isValid(imp, last) && isValid(exp, last) {
		consumption := imp.Daily() - exp.Daily()
		// Daily PV generation may be out of date, but it is used regardless.
		if pv_daily != nil {
			consumption += pv_daily.Daily()
		}
		val.Add("v3", fmt.Sprintf("%d", int(consumption*1000)))
		if p.d.Trace {
			log.Printf("v3 = %f, imp = %f, exp = %f", consumption, imp.Daily(), exp.Daily())
			if pv_daily != nil {
				log.Printf("daily = %f", pv_daily.Daily())
				if !isValid(pv_daily, last) {
					log.Printf("Using old generation data")
				}
			} else {
				log.Printf("No PV energy data\n")
			}
		}
	} else if *pvLog || p.d.Trace {
		if exp == nil {
			log.Printf("No export data\n")
		} else if !isValid(exp, last) {
			log.Printf("Export data not fresh\n")
		}
		if imp == nil {
			log.Printf("No import data\n")
		} else if !isValid(imp, last) {
			log.Printf("Import data not fresh\n")
		}
		log.Printf("No consumption data, v3 not updated\n")
	}
	tp, err := p.getPower(last)
	if err == nil {
		var g float64
		if isValid(pv_power, last) {
			g = pv_power.Get()
		}
		cp := int(g*1000 + tp)
		if cp >= 0 {
			val.Add("v4", fmt.Sprintf("%d", cp))
		} else {
			log.Printf("Negative power consumption (%d), v4 not updated, gen = %d, meter = %d\n", cp, int(g*1000), int(tp))
		}
		if p.d.Trace {
			if cp >= 0 {
				log.Printf("v4 = %d", cp)
			} else {
				log.Printf("power consumption = %d, not sent", cp)
			}
		}
	} else {
		log.Printf("Invalid total power, v4 not sent: %v\n", err)
	}
	req, err := http.NewRequest("POST", p.pvurl, strings.NewReader(val.Encode()))
	if err != nil {
		log.Printf("NewRequest failed: %v", err)
		return
	}
	if *pvLog {
		log.Printf("PV Uploading: %v", val)
	}
	req.Header.Add("X-Pvoutput-Apikey", p.key)
	req.Header.Add("X-Pvoutput-SystemId", p.id)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.d.Trace || *dryrun {
		log.Printf("req: %s (size %d)", val.Encode(), req.ContentLength)
		if *dryrun {
			return
		}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if p.d.Trace {
		log.Printf("Response is: %s", resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error: %s: %s", resp.Status, body)
	}
}

// getPower returns the current import/export power (as Watts)
func (p *pvWriter) getPower(last time.Time) (float64, error) {
	tp := p.d.GetElement(db.G_POWER)
	d_in := p.d.GetElement(db.D_IN_POWER)
	d_out := p.d.GetElement(db.D_OUT_POWER)
	if p.d.Trace {
		log.Printf("TP    = %f, valid = %v", tp.Get(), isValid(tp, last))
		log.Printf("IN-P  = %f, valid = %v", d_in.Get(), isValid(d_in, last))
		log.Printf("OUT-P = %f, valid = %v", d_out.Get(), isValid(d_out, last))
	}
	if isValid(tp, last) {
		return tp.Get() * 1000.0, nil
	}
	// Total power is not available, try the derived power.
	if isValid(d_in, last) && isValid(d_out, last) {
		return (d_in.Get() - d_out.Get()) * 1000.0, nil
	}
	return 0.0, fmt.Errorf("no valid power reading")
}

// isValid will return true if the element is not nil and has been updated
// in the last interval.
func isValid(e db.Element, last time.Time) bool {
	return e != nil && !e.Timestamp().Before(last)
}
