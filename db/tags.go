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

// A database of values is held in a map.  The values are identifed with strings called tags, listed below.
// A tag may have multiple inputs that may either be averaged or summed.
// These multiple inputs use the base tag string followed by an index value e.g TAG/0, TAG/1 etc.
// Gauges are prepended with 'G_', accumulators prepended with 'A_'.
// For example, 2 separate voltage inputs are identified as "VOLTS/0", "VOLTS/1", and the
// base tag of "VOLTS" is the average of the 2 inputs.
// If there were 2 separate PV generation values from 2 different inverters, these
// would be tagged as "GEN-D/0" and GEN-D/1", and the base tag of "GEN-D" will be the
// sum of the 2 inputs.
const (
	// Values read from meter.
	A_IN_TOTAL  = "IN"    // Total energy from grid to house (KwH)
	A_OUT_TOTAL = "OUT"   // Total energy from PV to grid (KwH)
	A_IMPORT    = "IMP"   // Sum of separate phases, energy from grid (KwH)
	A_EXPORT    = "EXP"   // Sum of separate phases, energy to grid (KwH)
	D_IN_POWER  = "IN-P"  // Derived In power (Kw)
	D_OUT_POWER = "OUT-P" // Derived Out power (Kw)
	// Values read from inverter,
	A_GEN_TOTAL = "GEN-T"   // Total energy generated from PV (KwH)
	A_GEN_DAILY = "GEN-D"   // Daily energy generated from PV (KWH)
	G_GEN_P     = "GEN-P"   // Current PV power (Kw)
	G_VOLTS     = "VOLTS"   // Current AC voltage (V)
	D_GEN_P     = "D-GEN-P" // Derived PV power (Kw)
	// Values read from weather service.
	G_TEMP = "TEMP" // Current temperature (degrees C)
	// Special values.
	C_TIME = "time" // Time checkpoint.
)
