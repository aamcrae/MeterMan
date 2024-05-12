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

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"

	_ "github.com/aamcrae/MeterMan/csv"
	"github.com/aamcrae/MeterMan/db"
	_ "github.com/aamcrae/MeterMan/hassi"
	_ "github.com/aamcrae/MeterMan/iammeter"
	// TODO Fix config for LCD.
	//	_ "github.com/aamcrae/MeterMan/meter"
	_ "github.com/aamcrae/MeterMan/pv"
	_ "github.com/aamcrae/MeterMan/server"
	_ "github.com/aamcrae/MeterMan/sma"
	_ "github.com/aamcrae/MeterMan/weather"
)

var configFile = flag.String("config", "", "Config file")
var profile = flag.Bool("profile", false, "Enable profiling")
var port = flag.Int("profileport", 6060, "Port for profiling server")
var logDate = flag.Bool("logtime", false, "Log date and time")

func main() {
	flag.Parse()
	if !*logDate {
		// Turn off date/time tags on logs
		log.SetFlags(0)
	}
	conf, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", *configFile, err)
	}
	if *profile {
		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf("localhost:%d", *port), nil))
		}()
	}
	d := db.NewDatabase(conf)
	err = d.Start()
	log.Fatalf("Initialisation error: %v", err)
}
