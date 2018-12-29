package main

import (
    "flag"
    "log"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
    _ "github.com/aamcrae/MeterMan/reader"
)


var conf = flag.String("config", ".meterman", "Config file")

func init() {
    flag.Parse()
}

func main() {
    conf, err := config.ParseFile(*conf)
    if err != nil {
        log.Fatalf("Can't read config %s: %v", *conf, err)
    }
    if err := core.SetUp(conf); err != nil {
        log.Fatalf("Initialisation error: %v", err)
    }
    var empty chan int = nil
    for {
        select {
        case <-empty:
        }
    }
}
