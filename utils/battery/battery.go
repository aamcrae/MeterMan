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
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/aldas/go-modbus-client"
	_ "github.com/aldas/go-modbus-client/packet"
)

var battery = flag.String("battery", "tcp://battery:502", "Battery address and port")
var unitId = flag.Int("id", 247, "Unit ID")

func init() {
	flag.Parse()
}

func main() {
	id := uint8(*unitId)

	b := modbus.NewRequestBuilder(*battery, id)

	requests, _ := b.
		AddField(modbus.Field{Name: "grid_sensor", Type: modbus.FieldTypeUint16, Address: 30004}).
		AddField(modbus.Field{Name: "grid_power", Type: modbus.FieldTypeInt32, Address: 30005}).
		AddField(modbus.Field{Name: "percent", Type: modbus.FieldTypeUint16, Address: 30014}).
		AddField(modbus.Field{Name: "power", Type: modbus.FieldTypeInt32, Address: 30037}).
		AddField(modbus.Field{Name: "state", Type: modbus.FieldTypeUint16, Address: 30051}).
		AddField(modbus.Field{Name: "max_charge", Type: modbus.FieldTypeUint32, Address: 30064}).
		AddField(modbus.Field{Name: "max_discharge", Type: modbus.FieldTypeUint32, Address: 30066}).
		AddField(modbus.Field{Name: "rated_charge", Type: modbus.FieldTypeUint32, Address: 30068}).
		AddField(modbus.Field{Name: "rated_discharge", Type: modbus.FieldTypeUint32, Address: 30070}).
		AddField(modbus.Field{Name: "acc_consume", Type: modbus.FieldTypeUint64, Address: 30094}).
		AddField(modbus.Field{Name: "batt_acc_charge", Type: modbus.FieldTypeUint64, Address: 30200}).
		AddField(modbus.Field{Name: "batt_acc_discharge", Type: modbus.FieldTypeUint64, Address: 30204}).
		ReadInputRegistersTCP()

	client := modbus.NewTCPClient()
	if err := client.Connect(context.Background(), *battery); err != nil {
		log.Fatalf("%s: connect %v", *battery, err)
	}
	defer client.Close()
	for _, req := range requests {
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			log.Fatalf("resp error: %v", err)
		}
		fields, _ := req.ExtractFields(resp, true)
		for _, f := range fields {
			fmt.Printf("%s: value %v\n", f.Field.Name, f.Value)
		}
	}
}
