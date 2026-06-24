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

type Field = int

const (
	F_GRID_POWER Field = iota
	F_PERCENT
	F_POWER
	F_MAX_CHARGE
	F_MAX_DISCHARGE
	F_ACC_CHARGE
	F_ACC_DISCHARGE
	F_LAST
)

const F_COUNT = int(F_LAST)

type Battery struct {
	Timeout time.Duration // Timeout
	Trace   bool
	addr    string

	requests []modbus.BuilderRequest
	client   *modbus.Client

	Values[F_COUNT] float64
}

var Fields = []struct {
	Name string
	Index Field
	mType modbus.FieldType
	addr uint16
	divisor float64
}{
	{ "grid_power", F_GRID_POWER, modbus.FieldTypeInt32, 30005, 1000.0},
	{ "percent", F_PERCENT, modbus.FieldTypeUint16, 30014, 10.0},
	{ "power", F_POWER, modbus.FieldTypeInt32, 30037, 1000.0},
	{ "max_charge", F_MAX_CHARGE, modbus.FieldTypeUint32, 30064, 100.0},
	{ "max_discharge", F_MAX_DISCHARGE, modbus.FieldTypeUint32, 30066, 100.0},
	{ "acc_charge", F_ACC_CHARGE, modbus.FieldTypeUint64, 30200, 100.0},
	{ "acc_discharge", F_ACC_DISCHARGE, modbus.FieldTypeUint64, 30204, 100.0},
}

var fieldMap map[string]int

func init() {
	fieldMap = make(map[string]int, len(Fields))
	for i, n := range Fields {
		fieldMap[n.Name] = i
	}
}

func NewBattery(addr string, unit uint8) (*Battery, error) {

	b := modbus.NewRequestBuilder(addr, unit)
	for _, f := range Fields {
		b.AddField(modbus.Field{Name: f.Name, Type: f.mType, Address: f.addr})
	}

	requests, err := b.ReadInputRegistersTCP()
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
			fi, ok := fieldMap[f.Field.Name]
			if !ok {
				return fmt.Errorf("unknown field name: %s", f.Field.Name)
			}
			ft := &Fields[fi]
			b.Values[ft.Index] = getValue(f.Value, ft.mType, ft.divisor)
		}
	}
	return nil
}

func getValue(value any, t modbus.FieldType, divisor float64) float64 {
	switch t {
	case modbus.FieldTypeInt16:
		return float64(value.(int16)) / divisor
	case modbus.FieldTypeUint16:
		return float64(value.(uint16)) / divisor
	case modbus.FieldTypeInt32:
		return float64(value.(int32)) / divisor
	case modbus.FieldTypeUint32:
		return float64(value.(uint32)) / divisor
	case modbus.FieldTypeUint64:
		return float64(value.(uint64)) / divisor
	default:
		panic(fmt.Sprintf("Unhandled modbus type: %v", t))
	}
}
