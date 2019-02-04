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

const weatherUrl = "http://api.openweathermap.org/data/2.5/weather?id=%s&units=metric&appid=%s"
const darkskyUrl = "https://api.darksky.net/forecast/%s/%s,%s?exclude=minutely,hourly,daily,alerts,flags&units=si"

var weatherpoll = flag.Int("weather-poll", 120, "Weather poll time (seconds)")

func init() {
	core.RegisterReader(weatherReader)
}

func weatherReader(conf *config.Config, wr chan<- core.Input) error {
	log.Printf("Registered temperature reader\n")
	service, err := conf.GetArg("tempservice")
	if err != nil {
		return err
	}
	var get func() (float64, error)
	switch service {
	default:
		return fmt.Errorf("%s: Unknown weather service", service)
	case "bom":
		url, err := conf.GetArg("bom")
		if err != nil {
			return err
		}
		get = func() (float64, error) {
			return BOM(url)
		}
	case "openweather":
		id, err := conf.GetArg("tempid")
		if err != nil {
			return err
		}
		key, err := conf.GetArg("tempkey")
		if err != nil {
			return err
		}
		url := fmt.Sprintf(weatherUrl, id, key)
		get = func() (float64, error) {
			return OpenWeather(url)
		}
	case "darksky":
		key, err := conf.GetArg("darkskykey")
		if err != nil {
			return err
		}
		lat, err := conf.GetArg("darkskylat")
		if err != nil {
			return err
		}
		lng, err := conf.GetArg("darkskylong")
		if err != nil {
			return err
		}
		url := fmt.Sprintf(darkskyUrl, key, lat, lng)
		get = func() (float64, error) {
			return Darksky(url)
		}
	}
	core.AddGauge(core.G_TEMP)
	go reader(get, wr)
	return nil
}

func reader(get func() (float64, error), wr chan<- core.Input) {
	for {
		t, err := get()
		if err != nil {
			log.Printf("Getting temperature: %v\n", err)
		} else {
			if *core.Verbose {
				log.Printf("Current temperature: %f\n", t)
			}
			wr <- core.Input{core.G_TEMP, t}
		}
		time.Sleep(time.Duration(*weatherpoll) * time.Second)
	}
}

func OpenWeather(url string) (float64, error) {
	body, err := fetch(url)
	if err != nil {
		return 0, err
	}
	type Main struct {
		Temp float64
	}
	type resp struct {
		Main    Main
		Cod     int
		Message string
	}
	var m resp
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, err
	}
	if m.Cod != 200 {
		return 0, fmt.Errorf("Response %d: %s", m.Cod, m.Message)
	}
	return m.Main.Temp, nil
}

func Darksky(url string) (float64, error) {
	body, err := fetch(url)
	if err != nil {
		return 0, err
	}
	type Currently struct {
		Temp float64 `json:"temperature"`
		Aparent float64 `json:"apparentTemperature"`
	}
	type resp struct {
		Currently    Currently
	}
	var m resp
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, err
	}
	return m.Currently.Temp, nil
}

func BOM(url string) (float64, error) {
	body, err := fetch(url)
	if err != nil {
		return 0, err
	}
	type Data struct {
		Apparant float64 `json:"apparent_t"`
		Air      float64 `json:"air_temp"`
	}
	type Ob struct {
		Data []*Data
	}
	type resp struct {
		Observations *Ob
	}
	var m resp
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, err
	}
	if m.Observations == nil || len(m.Observations.Data) == 0 {
		return 0, fmt.Errorf("BOM: Bad response")
	}
	return m.Observations.Data[0].Air, nil
}

func fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
