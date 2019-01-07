package temp

import (
	"encoding/json"
	"flag"
	"fmt"
    "io/ioutil"
	"log"
	"net/http"
    "time"

	"github.com/aamcrae/MeterMan/core"
	"github.com/aamcrae/config"
)

const kelvinBase = 275.15

var weatherpoll = flag.Int("weather-poll", 120, "Weather poll time (seconds)")

type Main struct {
    Temp float64
}

type Blob struct {
    Main Main
    Cod int
    Message string
}

func init() {
	core.RegisterReader(weatherReader)
}

func weatherReader(conf *config.Config, wr chan<- core.Input) error {
	log.Printf("Registered temperature reader\n")
	url, err := conf.GetArg("weather")
	if err != nil {
		return err
	}
	core.AddGauge(core.G_TEMP)
	go reader(url, wr)
	return nil
}

func reader(url string, wr chan<- core.Input) {
	for {
		time.Sleep(time.Duration(*weatherpoll) * time.Second)
        t, err := GetTemp(url)
        if err != nil {
            log.Printf("Getting temperature: %v\n", err)
        } else {
	        wr <- core.Input{core.G_TEMP, t}
        }
	}
}

func GetTemp(url string) (float64, error) {
    resp, err := http.Get(url)
    if err != nil {
	    return 0, err
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return 0, err
    }
    var m Blob
    if err := json.Unmarshal(body, &m); err != nil {
        return 0, err
    }
    if m.Cod != 200 {
        return 0, fmt.Errorf("Response %d: %s", m.Cod, m.Message)
    }
    return m.Main.Temp - kelvinBase, nil
}
