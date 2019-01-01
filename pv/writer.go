package pv

import (
    "flag"
    "fmt"
    "io/ioutil"
	"log"
	"net/http"
    "net/url"
    "strings"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)

var dryrun = flag.Bool("dryrun", false, "Do not upload data")

var apikey string
var systemid string
var serverUrl string

func init() {
    core.RegisterWriter(pvoutputInit)
}

func pvoutputInit(conf *config.Config, data <-chan *core.Output) error {
    log.Printf("Registered pvoutput uploader as writer\n")
    if a, err := conf.GetArg("apikey"); err != nil {
        return err
    } else {
        apikey = a
    }
    if a, err := conf.GetArg("systemid"); err != nil {
        return err
    } else {
        systemid = a
    }
    if a, err := conf.GetArg("pvurl"); err != nil {
        return err
    } else {
        serverUrl = a
    }
    go writer(data)
    return nil
}

func writer(data <-chan *core.Output) {
    for {
        d := <-data
        tp := getGauge(d, core.G_TP)
        genp := getGauge(d, core.G_GEN_P)
        volts := getGauge(d, core.G_VOLTS)
        daily := getAccum(d, core.A_GEN_DAILY)
        imp := getAccum(d, core.A_IN_TOTAL)
        exp := getAccum(d, core.A_OUT_TOTAL)

        val := url.Values{}
        val.Add("d", d.Time.Format("20060102"))
        val.Add("t", d.Time.Format("15:04"))
        if daily != nil {
            val.Add("v1", fmt.Sprintf("%d", int(daily.Get() * 1000)))
            if *core.Verbose {
                log.Printf("v1 = %f", daily.Get())
            }
        }
        if genp != nil {
            val.Add("v2", fmt.Sprintf("%d", int(genp.Get() * 1000)))
            if *core.Verbose {
                log.Printf("v2 = %f", genp.Get())
            }
        }
        if imp != nil && exp != nil && daily != nil {
            consumption := imp.Daily() + daily.Get() - exp.Daily()
            val.Add("v2", fmt.Sprintf("%d", int(consumption * 1000)))
            if *core.Verbose {
                log.Printf("v2 = %f", consumption)
            }
        }
        if tp != nil && genp != nil {
            val.Add("v4", fmt.Sprintf("%d", int((genp.Get() + tp.Get()) * 1000)))
            if *core.Verbose {
                log.Printf("v4 = %f", genp.Get() + tp.Get())
            }
        }
        if volts != nil {
            val.Add("v6", fmt.Sprintf("%.2f", volts.Get()))
            if *core.Verbose {
                log.Printf("v6 = %.2f", volts.Get())
            }
        }
        req, err := http.NewRequest("POST", serverUrl, strings.NewReader(val.Encode()))
        if err != nil {
            log.Printf("NewRequest failed: %v", err)
            continue
        }
        req.Header.Add("X-Pvoutput-Apikey", apikey)
        req.Header.Add("X-Pvoutput-SystemId", systemid)
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
        if *core.Verbose || *dryrun {
            log.Printf("req: %s (size %d)", val.Encode(), req.ContentLength)
            if *dryrun {
                continue
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
            continue
        }
    }
}

func getGauge(d *core.Output, name string) (*core.Gauge) {
    el, ok := d.Values[name]
    if !ok {
        return nil
    }
    return el.(*core.Gauge)
}

func getAccum(d *core.Output, name string) (*core.Accum) {
    el, ok := d.Values[name]
    if !ok {
        return nil
    }
    return el.(*core.Accum)
}
