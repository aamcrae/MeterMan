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

var apikey string
var systemid string
var serverUrl string

func init() {
	core.RegisterWriter(pvoutputInit)
}

func pvoutputInit(conf *config.Config) (func(time.Time), error) {
	log.Printf("Registered pvoutput uploader as writer\n")
	if a, err := conf.GetArg("apikey"); err != nil {
		return nil, err
	} else {
		apikey = a
	}
	if a, err := conf.GetArg("systemid"); err != nil {
		return nil, err
	} else {
		systemid = a
	}
	if a, err := conf.GetArg("pvurl"); err != nil {
		return nil, err
	} else {
		serverUrl = a
	}
	return writer, nil
}

func writer(t time.Time) {
	tp := core.GetElement(core.G_TP)
	pv_power := core.GetElement(core.G_GEN_P)
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
		// Daily PV generation may be out of date.
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
	req, err := http.NewRequest("POST", serverUrl, strings.NewReader(val.Encode()))
	if err != nil {
		log.Printf("NewRequest failed: %v", err)
		return
	}
	req.Header.Add("X-Pvoutput-Apikey", apikey)
	req.Header.Add("X-Pvoutput-SystemId", systemid)
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
