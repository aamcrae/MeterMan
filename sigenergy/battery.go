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
	"fmt"
	"time"

	"github.com/aldas/go-modbus-client"
)

type Battery struct {
	Timeout time.Duration // Timeout
	Trace   bool
	addr    string

	requests []modbus.BuilderRequest
	client   *modbus.Client
	indexMap map[string]int
	values   []*float64

	GridPower    float64
	Percent      float64
	Power        float64
	AccCharge    float64
	AccDischarge float64
}

var fields = []struct {
	name    string
	mType   modbus.FieldType
	addr    uint16
	divisor float64
}{
	{"grid_power", modbus.FieldTypeInt32, 30005, 1000.0},
	{"percent", modbus.FieldTypeUint16, 30014, 10.0},
	{"power", modbus.FieldTypeInt32, 30037, 1000.0},
	{"acc_charge", modbus.FieldTypeUint64, 30200, 100.0},
	{"acc_discharge", modbus.FieldTypeUint64, 30204, 100.0},
}

func NewBattery(addr string, unit uint8) (*Battery, error) {
	batt := &Battery{
		Timeout:  time.Second * 10,
		Trace:    false,
		addr:     addr,
		indexMap: make(map[string]int, len(fields)),
		values:   make([]*float64, len(fields)),
	}

	b := modbus.NewRequestBuilder(addr, unit)
	for i, f := range fields {
		batt.indexMap[f.name] = i
		b.AddField(modbus.Field{Name: f.name, Type: f.mType, Address: f.addr})
	}

	var err error
	batt.requests, err = b.ReadInputRegistersTCP()
	if err != nil {
		return nil, err
	}

	batt.client = modbus.NewTCPClient()
	batt.values[0] = &batt.GridPower
	batt.values[1] = &batt.Percent
	batt.values[2] = &batt.Power
	batt.values[3] = &batt.AccCharge
	batt.values[4] = &batt.AccDischarge
	return batt, nil
}

func (b *Battery) Poll() error {
	if err := b.client.Connect(context.Background(), b.addr); err != nil {
		return fmt.Errorf("connect to %s: %w", b.addr, err)
	}
	defer b.client.Close()
	for _, req := range b.requests {
		resp, err := b.client.Do(context.Background(), req)
		if err != nil {
			return fmt.Errorf("req failed: %w", err)
		}
		results, _ := req.ExtractFields(resp, true)
		for _, f := range results {
			fi, ok := b.indexMap[f.Field.Name]
			if !ok {
				return fmt.Errorf("unknown field name: %s", f.Field.Name)
			}
			ft := &fields[fi]
			*b.values[fi] = getValue(f.Value, ft.mType) / ft.divisor
		}
	}
	return nil
}

func getValue(value any, t modbus.FieldType) float64 {
	switch t {
	case modbus.FieldTypeInt16:
		return float64(value.(int16))
	case modbus.FieldTypeUint16:
		return float64(value.(uint16))
	case modbus.FieldTypeInt32:
		return float64(value.(int32))
	case modbus.FieldTypeUint32:
		return float64(value.(uint32))
	case modbus.FieldTypeUint64:
		return float64(value.(uint64))
	default:
		panic(fmt.Sprintf("Unhandled modbus type: %v", t))
	}
}
