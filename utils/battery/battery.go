// Copyright 2025 Andrew McRae
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

	batt "github.com/aamcrae/MeterMan/sigenergy"
)

var battery = flag.String("battery", "tcp://battery:502", "Battery address and port")
var unitId = flag.Int("id", 247, "Unit ID")

func init() {
	flag.Parse()
}

func main() {
	id := uint8(*unitId)

	b, err := batt.NewBattery(*battery, id)
	if err != nil {
		log.Fatalf("%s: create %v", *battery, err)
	}
	err = b.Poll()
	if err != nil {
		log.Fatalf("%s: poll %v", *battery, err)
	}
	fmt.Printf("Grid power: %f\n", b.GridPower)
	fmt.Printf("     Power: %f\n", b.Power)
	fmt.Printf("   Percent: %f\n", b.Percent)
	fmt.Printf("Acc charge: %f\n", b.AccCharge)
	fmt.Printf(" discharge: %f\n", b.AccDischarge)
}
