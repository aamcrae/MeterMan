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
	"time"

	"testing"
)

func TestGauge(t *testing.T) {
	g := NewGauge("100.0 10", time.Minute*10)
	v := g.Get()
	if !cmp(v, 100) {
		t.Errorf("NewGauge: got %v want %v\n", v, 100.0)
	}
	ts := g.Timestamp()
	if ts != time.Unix(10, 0) {
		t.Errorf("NewGauge: got %v want %v\n", ts, 10)
	}
	g.Update(200.0, time.Unix(20, 0))
	ts = g.Timestamp()
	if ts != time.Unix(20, 0) {
		t.Errorf("NewGauge: got %v want %v\n", ts, 20)
	}
	v = g.Get()
	if !cmp(v, 200) {
		t.Errorf("NewGauge: got %v want %v\n", v, 200.0)
	}
	g.Update(100.0, time.Unix(30, 0))
	v = g.Get()
	if !cmp(v, 100) {
		t.Errorf("NewGauge: got %v want %v\n", v, 100.0)
	}
}

func cmp(f1, f2 float64) bool {
	const tolerance = 0.001 // Floating point comparison to 0.1%
	if f1 == f2 {
		return true
	}
	if f1 == 0 || f2 == 0 {
		return false
	}
	d := math.Abs(f1 - f2)
	return math.Abs(d/f1) < tolerance
}
