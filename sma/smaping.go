package main

import (
    "flag"
    "log"

    "github.com/aamcrae/MeterMan/sma"
)

var inverter = flag.string("inverter", "meter:9522", "Inverter address and port")


func init() {
    flag.Parse()
}

func main() {

    sma, err := sma.NewSMA(*inverter)
    if err != nil {
        log.Fatalf("%s: %v", *inverter, err)
    }
    b, err := sma.Query(
    if err != nil {
        log.Fatalf("Query: %v", err)
    }
    log.Printf("len = %d, buf %v\n", b.Len(), b.Bytes())
}
