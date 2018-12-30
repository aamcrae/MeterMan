package main

import (
    "flag"
    "log"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
    _ "github.com/aamcrae/MeterMan/lcd"
    _ "github.com/aamcrae/MeterMan/pv"
    _ "github.com/aamcrae/MeterMan/sma"
)


var conf = flag.String("config", ".meterman", "Config file")

func main() {
    flag.Parse()
    conf, err := config.ParseFile(*conf)
    if err != nil {
        log.Fatalf("Can't read config %s: %v", *conf, err)
    }
    err = core.SetUpAndRun(conf)
    log.Fatalf("Initialisation error: %v", err)
}
