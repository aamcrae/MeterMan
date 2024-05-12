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

// package weather extracts current weather data from selected providers.
// The package is configured as a section in the YAML config file:
//   weather:
//     tempservice: {bom,openweather,darksky}  # Choose one
//
// if bom:
//     bom: <URL of JSON output for location>
// if openweather:
//     tempid: <openweather id for locaton>
//     tempkey: <openweather API key>
// if darksky:
//     darkskykey: <darksky API key>
//     darkskylat: <location latitude>
//     darkskylong: <location longitude>

package weather

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

const weatherUrl = "http://api.openweathermap.org/data/2.5/weather?id=%s&units=metric&appid=%s"
const darkskyUrl = "https://api.darksky.net/forecast/%s/%s,%s?exclude=minutely,hourly,daily,alerts,flags&units=si"

var weatherpoll = flag.Int("weather-poll", 120, "Weather poll time (seconds)")

type Weather struct {
	Tempservice string
	Bom         string
	Tempid      string
	Tempkey     string
	Darkskykey  string
	Darkskylat  string
	Darkskylong string
}

func init() {
	db.RegisterInit(weatherReader)
}

func weatherReader(d *db.DB) error {
	var conf Weather
	dec, ok := d.Config["weather"]
	if !ok {
		return nil
	}
	err := dec.Decode(&conf)
	if err != nil {
		return err
	}
	var get func() (float64, error)
	switch conf.Tempservice {
	default:
		return fmt.Errorf("%s: Unknown weather service", conf.Tempservice)
	case "bom":
		get = func() (float64, error) {
			return BOM(conf.Bom)
		}
	case "openweather":
		url := fmt.Sprintf(weatherUrl, conf.Tempid, conf.Tempkey)
		get = func() (float64, error) {
			return OpenWeather(url)
		}
	case "darksky":
		url := fmt.Sprintf(darkskyUrl, conf.Darkskykey, conf.Darkskylat, conf.Darkskylong)
		get = func() (float64, error) {
			return Darksky(url)
		}
	}
	log.Printf("Registered temperature reader using service %s, polling every %d seconds\n", conf.Tempservice, *weatherpoll)
	d.AddGauge(db.G_TEMP)
	if !d.Dryrun {
		go reader(d, get)
	}
	return nil
}

func reader(d *db.DB, get func() (float64, error)) {
	for {
		t, err := get()
		if err != nil {
			log.Printf("Getting temperature: %v\n", err)
		} else {
			if d.Trace {
				log.Printf("Current temperature: %g\n", t)
			}
			d.Input(db.G_TEMP, t)
		}
		time.Sleep(time.Duration(*weatherpoll) * time.Second)
	}
}

func OpenWeather(url string) (float64, error) {
	type Main struct {
		Temp float64
	}
	type resp struct {
		Main    Main
		Cod     int
		Message string
	}
	var m resp
	err := fetch(url, &m)
	if err != nil {
		return 0, err
	}
	if m.Cod != 200 {
		return 0, fmt.Errorf("Response %d: %s", m.Cod, m.Message)
	}
	return m.Main.Temp, nil
}

func Darksky(url string) (float64, error) {
	type Currently struct {
		Temp    float64 `json:"temperature"`
		Aparent float64 `json:"apparentTemperature"`
	}
	type resp struct {
		Currently Currently
	}
	var m resp
	err := fetch(url, &m)
	if err != nil {
		return 0, err
	}
	return m.Currently.Temp, nil
}

func BOM(url string) (float64, error) {
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
	err := fetch(url, &m)
	if err != nil {
		return 0, err
	}
	if m.Observations == nil || len(m.Observations.Data) == 0 {
		return 0, fmt.Errorf("BOM: Bad response")
	}
	return m.Observations.Data[0].Air, nil
}

func fetch(url string, m interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, m)
}
