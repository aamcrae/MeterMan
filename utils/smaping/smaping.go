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
	"log"

	"github.com/aamcrae/MeterMan/sma"
)

var inverter = flag.String("inverter", "inverter:9522", "Inverter address and port")
var password = flag.String("password", "", "Inverter password")
var getall = flag.Bool("all", false, "Get all records")

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
	day, err := sma.DailyEnergy()
	if err != nil {
		log.Printf("Daily Energy: %v", err)
	}
	total, err := sma.TotalEnergy()
	if err != nil {
		log.Printf("Total Energy: %v", err)
	}
	log.Printf("day = %f KwH, total = %f KwH\n", day, total)
	p, err := sma.Power()
	if err != nil {
		log.Fatalf("Power: %v", err)
	}
	log.Printf("power = %f\n", p)
	v, err := sma.Voltage()
	if err != nil {
		log.Fatalf("Voltage: %v", err)
	}
	log.Printf("volts = %f\n", v)
	if *getall {
		sma.GetAll()
	}
}
