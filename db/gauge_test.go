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

package db

import (
	"math"

	"testing"
)

const tolerance = 0.001 // Floating point comparison to 0.1%

func TestGauge(t *testing.T) {
	g := NewGauge("100.0")
	v := g.Get()
	if !cmp(v, 100) {
		t.Errorf("NewGauge: got %v want %v\n", v, 100.0)
	}
	u := g.Updated()
	if u {
		t.Errorf("NewGauge: got %v want %v\n", u, false)
	}
	g.Update(200.0)
	u = g.Updated()
	if !u {
		t.Errorf("NewGauge: got %v want %v\n", u, true)
	}
	v = g.Get()
	if !cmp(v, 200) {
		t.Errorf("NewGauge: got %v want %v\n", v, 200.0)
	}
	g.Update(100.0)
	v = g.Get()
	if !cmp(v, 150) {
		t.Errorf("NewGauge: got %v want %v\n", v, 150.0)
	}
	g.ClearUpdate()
	u = g.Updated()
	if u {
		t.Errorf("NewGauge: got %v want %v\n", u, false)
	}
	v = g.Get()
	if !cmp(v, 150) {
		t.Errorf("NewGauge: got %v want %v\n", v, 150.0)
	}
}

func cmp(f1, f2 float64) bool {
	if f1 == f2 {
		return true
	}
	if f1 == 0 || f2 == 0 {
		return false
	}
	d := math.Abs(f1 - f2)
	return math.Abs(d/f1) < tolerance
}
