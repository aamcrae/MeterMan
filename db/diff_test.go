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

func TestDiff(t *testing.T) {
	g := NewDiff("10.0 100.0 10")
	v := g.Get()
	if !diffCmp(v, 10) {
		t.Errorf("Diff: got %v want %v\n", v, 10.0)
	}
	ts := g.Timestamp()
	if ts != time.Unix(10, 0) {
		t.Errorf("Diff: got %v want %v\n", ts, 10)
	}
    // Add 1KwH in 1 minute.
	g.Update(101.0, time.Unix(10+60, 0))
	ts = g.Timestamp()
	if ts != time.Unix(70, 0) {
		t.Errorf("Diff: got %v want %v\n", ts, 70)
	}
	// Should have 60 KwH
	v = g.Get()
	if !diffCmp(v, 60) {
		t.Errorf("Diff: got %v want %v\n", v, 60.0)
	}
	// Add 4KwH in 2 minutes
	g.Update(105.0, time.Unix(70+120, 0))
	v = g.Get()
	if !diffCmp(v, 120) {
		t.Errorf("Diff: got %v want %v\n", v, 120.0)
	}
}

func diffCmp(f1, f2 float64) bool {
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
