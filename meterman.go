package main

import (
	"flag"
	"log"

	"github.com/aamcrae/MeterMan/core"
	_ "github.com/aamcrae/MeterMan/csv"
	_ "github.com/aamcrae/MeterMan/lcd"
	_ "github.com/aamcrae/MeterMan/pv"
	_ "github.com/aamcrae/MeterMan/sma"
	_ "github.com/aamcrae/MeterMan/temp"
	"github.com/aamcrae/config"
)

var conf = flag.String("config", "/etc/meterman.conf", "Config file")

func main() {
	flag.Parse()
	conf, err := config.ParseFile(*conf)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", *conf, err)
	}
	err = core.SetUpAndRun(conf)
	log.Fatalf("Initialisation error: %v", err)
}
