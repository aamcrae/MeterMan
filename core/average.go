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

package core

import (
	"fmt"
	"time"
)

type Average struct {
	value   float64
	acc     float64
	current float64
	last    time.Time
	updated bool
}

func NewAverage(cp string) *Average {
	g := new(Average)
	fmt.Sscanf(cp, "%f", &g.current)
	g.last = time.Now()
	g.value = g.current
	return g
}

func (g *Average) Update(value float64) {
	g.current = value
	g.acc += time.Now().Sub(g.last).Seconds() * g.current
	g.updated = true
}

func (g *Average) Interval(t time.Time, midnight bool) {
	g.acc += t.Sub(g.last).Seconds() * g.current
	g.value = g.acc / interval.Seconds()
	g.acc = 0
	g.last = t
}

func (g *Average) Get() float64 {
	return g.value
}

func (g *Average) Updated() bool {
	return g.updated
}

func (g *Average) ClearUpdate() {
	g.updated = false
}

func (g *Average) Checkpoint() string {
	return fmt.Sprintf("%f", g.value)
}
