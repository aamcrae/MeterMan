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

package sigenergy

import (
	"context"
	"log"
	"time"

	"github.com/aldas/go-modbus-client"
)

type Battery struct {
	Timeout time.Duration // Timeout
	Trace   bool
	addr    string

	requests []modbus.BuilderRequest
	client   *modbus.Client

	grid_power    float64
	percent       float64
	power         float64
	max_charge    float64
	max_discharge float64
	acc_charge    float64
	acc_discharge float64
}

func NewBattery(addr string, unit uint8) (*Battery, error) {

	b := modbus.NewRequestBuilder(addr, unit)

	requests, err := b.
		AddField(modbus.Field{Name: "grid_sensor", Type: modbus.FieldTypeUint16, Address: 30004}).
		AddField(modbus.Field{Name: "grid_power", Type: modbus.FieldTypeInt32, Address: 30005}).
		AddField(modbus.Field{Name: "percent", Type: modbus.FieldTypeUint16, Address: 30014}).
		AddField(modbus.Field{Name: "power", Type: modbus.FieldTypeInt32, Address: 30037}).
		AddField(modbus.Field{Name: "state", Type: modbus.FieldTypeUint16, Address: 30051}).
		AddField(modbus.Field{Name: "max_charge", Type: modbus.FieldTypeUint32, Address: 30064}).
		AddField(modbus.Field{Name: "max_discharge", Type: modbus.FieldTypeUint32, Address: 30066}).
		AddField(modbus.Field{Name: "acc_charge", Type: modbus.FieldTypeUint64, Address: 30200}).
		AddField(modbus.Field{Name: "acc_discharge", Type: modbus.FieldTypeUint64, Address: 30204}).
		ReadInputRegistersTCP()
	if err != nil {
		return nil, err
	}

	client := modbus.NewTCPClient()
	return &Battery{
		Timeout:  time.Second * 10,
		Trace:    false,
		addr:     addr,
		requests: requests,
		client:   client,
	}, nil
}

func (b *Battery) poll() error {
	if err := b.client.Connect(context.Background(), b.addr); err != nil {
		return err
	}
	defer b.client.Close()
	findex := 0
	for _, req := range b.requests {
		resp, err := b.client.Do(context.Background(), req)
		if err != nil {
			return err
		}
		fields, _ := req.ExtractFields(resp, true)
		for _, f := range fields {
			switch findex {
			default:
			case 1:
				b.grid_power = float64(f.Value.(int32)) / 1000.0
			case 2:
				b.percent = float64(f.Value.(uint16)) / 10.0
			case 3:
				b.power = float64(f.Value.(int32)) / 1000.0
			case 5:
				b.max_charge = float64(f.Value.(uint32)) / 100.0
			case 6:
				b.max_discharge = float64(f.Value.(uint32)) / 100.0
			case 7:
				b.acc_charge = float64(f.Value.(uint64)) / 100.0
			case 8:
				b.acc_discharge = float64(f.Value.(uint64)) / 100.0
			}
			findex++
		}
	}
	return nil
}
