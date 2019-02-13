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
		log.Fatalf("Can't read config %s: %v", *conf, err)
	}
	if *profile {
		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf("localhost:%d", *port), nil))
		}()
	}
	err = core.SetUpAndRun(conf)
	log.Fatalf("Initialisation error: %v", err)
}
