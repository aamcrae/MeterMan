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

	"github.com/aamcrae/MeterMan/core"
	"github.com/aamcrae/config"
)

var dryrun = flag.Bool("dryrun", false, "Do not upload data")

func init() {
	core.RegisterWriter(pvoutputInit)
}

func pvoutputInit(conf *config.Config) (func(time.Time), error) {
	sect := conf.GetSection("pvoutput")
	if sect == nil {
		return nil, nil
	}
	key, err := sect.GetArg("apikey")
	if err != nil {
		return nil, err
	}
	id, err := sect.GetArg("systemid")
	if err != nil {
		return nil, err
	}
	pvurl, err := sect.GetArg("pvurl")
	if err != nil {
		return nil, err
	}
	log.Printf("Registered pvoutput uploader as writer\n")
	return func(t time.Time) {
		writer(t, pvurl, id, key)
	}, nil
}

// writer creates a post request to pvoutput.org to upload the current data.
func writer(t time.Time, pvurl, id, key string) {
	tp := core.GetElement(core.G_TP)
	pv_power := core.GetElement(core.G_GEN_P)
	temp := core.GetElement(core.G_TEMP)
	volts := core.GetElement(core.G_VOLTS)
	pv_daily := core.GetAccum(core.A_GEN_TOTAL)
	imp := core.GetAccum(core.A_IN_TOTAL)
	exp := core.GetAccum(core.A_OUT_TOTAL)
	hour := t.Hour()
	daytime := hour >= *core.StartHour && hour < *core.EndHour

	val := url.Values{}
	val.Add("d", t.Format("20060102"))
	val.Add("t", t.Format("15:04"))
	if pv_daily != nil && pv_daily.Updated() && daytime {
		val.Add("v1", fmt.Sprintf("%d", int(pv_daily.Daily()*1000)))
		if *core.Verbose {
			log.Printf("v1 = %f", pv_daily.Daily())
		}
	} else if *core.Verbose {
		if pv_daily == nil {
			log.Printf("No PV energy total, v1 not updated\n")
		} else {
			log.Printf("PV Energy not fresh, v1 not updated\n")
		}
	}
	if pv_power != nil && pv_power.Updated() {
		val.Add("v2", fmt.Sprintf("%d", int(pv_power.Get()*1000)))
		if *core.Verbose {
			log.Printf("v2 = %f", pv_power.Get())
		}
	} else if *core.Verbose {
		log.Printf("No PV power, v2 not updated\n")
	}
	if temp != nil && temp.Updated() && temp.Get() != 0 {
		val.Add("v5", fmt.Sprintf("%.2f", temp.Get()))
		if *core.Verbose {
			log.Printf("v5 = %.2f", temp.Get())
		}
	} else if *core.Verbose {
		log.Printf("No temperature, v5 not updated\n")
	}
	if volts != nil && volts.Updated() && volts.Get() != 0 {
		val.Add("v6", fmt.Sprintf("%.2f", volts.Get()))
		if *core.Verbose {
			log.Printf("v6 = %.2f", volts.Get())
		}
	} else if *core.Verbose {
		log.Printf("No Voltage, v6 not updated\n")
	}
	if imp != nil && imp.Updated() && exp != nil && exp.Updated() {
		consumption := imp.Daily() - exp.Daily()
		// Daily PV generation may be out of date, but it is used regardless.
		if pv_daily != nil {
			consumption += pv_daily.Daily()
		}
		val.Add("v3", fmt.Sprintf("%d", int(consumption*1000)))
		if *core.Verbose {
			log.Printf("v3 = %f, imp = %f, exp = %f", consumption, imp.Daily(), exp.Daily())
			if pv_daily != nil {
				log.Printf("daily = %f", pv_daily.Daily())
				if !pv_daily.Updated() {
					log.Printf("Using old generation data")
				}
			} else {
				log.Printf("No PV energy data\n")
			}
		}
	} else if *core.Verbose {
		if exp == nil {
			log.Printf("No export data\n")
		} else if !exp.Updated() {
			log.Printf("Export data not fresh\n")
		}
		if imp == nil {
			log.Printf("No import data\n")
		} else if !imp.Updated() {
			log.Printf("Import data not fresh\n")
		}
		log.Printf("No consumption data, v3 not updated\n")
	}
	if tp != nil && tp.Updated() {
		var g float64
		if pv_power != nil && pv_power.Updated() {
			g = pv_power.Get()
		}
		val.Add("v4", fmt.Sprintf("%d", int((g+tp.Get())*1000)))
		if *core.Verbose {
			log.Printf("v4 = %f", g+tp.Get())
		}
	} else if *core.Verbose {
		if tp == nil {
			log.Printf("No total power, v4 not updated\n")
		} else if !tp.Updated() {
			log.Printf("Total not fresh, v4 not updated\n")
		}
	}
	req, err := http.NewRequest("POST", pvurl, strings.NewReader(val.Encode()))
	if err != nil {
		log.Printf("NewRequest failed: %v", err)
		return
	}
	req.Header.Add("X-Pvoutput-Apikey", key)
	req.Header.Add("X-Pvoutput-SystemId", id)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if *core.Verbose || *dryrun {
		log.Printf("req: %s (size %d)", val.Encode(), req.ContentLength)
		if *dryrun {
			return
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if *core.Verbose {
		log.Printf("Response is: %s", resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error: %s: %s", resp.Status, body)
		return
	}
}
