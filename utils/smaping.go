package main

import (
    "flag"
    "log"

    "github.com/aamcrae/MeterMan/sma"
)

var inverter = flag.String("inverter", "inverter:9522", "Inverter address and port")
var password = flag.String("password", "", "Inverter password")


func init() {
    flag.Parse()
}

func main() {

    sma, err := sma.NewSMA(*inverter, *password)
    if err != nil {
        log.Fatalf("%s: %v", *inverter, err)
    }
    id, serial, err := sma.Logon()
    if err != nil {
        log.Fatalf("Logon: %v", err)
    }
    defer sma.Logoff()
    log.Printf("ID = %d, serial number = %d\n", id, serial)
    stat, err := sma.DeviceStatus()
    if err != nil {
        log.Fatalf("DeviceStatus: %v", err)
    }
    log.Printf("status = %s\n", stat)
    day, total, err := sma.Energy()
    if err != nil {
        log.Fatalf("Energy: %v", err)
    }
    log.Printf("day = %f KwH, total = %f KwH\n", day, total)
    p, err := sma.Power()
    if err != nil {
        log.Fatalf("Power: %v", err)
    }
    log.Printf("power = %d\n", p)
    v, err := sma.Voltage()
    if err != nil {
        log.Fatalf("Voltage: %v", err)
    }
    log.Printf("volts = %f\n", v)
}
