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
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/aamcrae/MeterMan/core"
	_ "github.com/aamcrae/MeterMan/csv"
	_ "github.com/aamcrae/MeterMan/lcd"
	_ "github.com/aamcrae/MeterMan/pv"
	_ "github.com/aamcrae/MeterMan/sma"
	_ "github.com/aamcrae/MeterMan/weather"
	"github.com/aamcrae/config"
)

var configFile = flag.String("config", "/etc/meterman.conf", "Config file")
var profile = flag.Bool("profile", false, "Enable profiling")
var port = flag.Int("port", 6060, "Port for http server")

func main() {
	flag.Parse()
	conf, err := config.ParseFile(*configFile)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", *configFile, err)
	}
	if *profile {
		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf("localhost:%d", *port), nil))
		}()
	}
	err = core.SetUpAndRun(conf)
	log.Fatalf("Initialisation error: %v", err)
}
