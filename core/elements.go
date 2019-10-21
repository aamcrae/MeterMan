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

// A database of values is held in a map.  The values are identifed with strings called tags, listed below.
// A single value may have multiple inputs that could be averaged or accumulated together.
// These multiple inputs use the base tag string followed by an index value e.g TAG/0, TAG/1 etc.
// Gauges are prepended with 'G_', accumulators prepended with 'A_'.
const (
	// Values read from meter.
	A_IN_TOTAL  = "IN"  // Total energy from grid to house (KwH)
	A_OUT_TOTAL = "OUT" // Total energy from PV to grid (KwH)
	A_IMPORT    = "IMP" // Sum of separate phases, energy from grid (KwH)
	A_EXPORT    = "EXP" // Sum of separate phases, energy to grid (KwH)
	G_TP        = "TP"  // Current import/export power (-ve is export to grid) (Kw)
	// Values read from inverter,
	A_GEN_TOTAL = "GEN-T" // Total energy generated from PV (KwH)
	A_GEN_DAILY = "GEN-D" // Daily energy generated from PV (KWH)
	G_GEN_P     = "GEN-P" // Current PV power (Kw)
	G_VOLTS     = "VOLTS" // Current AC voltage (V)
	// Values read from weather service.
	G_TEMP = "TEMP" // Current temperature
	// Special values.
	C_TIME = "time" // Time checkpoint.
)
