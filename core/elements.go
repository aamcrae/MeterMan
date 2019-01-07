package core

import (
	"flag"
)

var StartHour = flag.Int("starthour", 6, "Start hour for PV (e.g 6)")
var EndHour = flag.Int("endhour", 20, "End hour for PV (e.g 19)")

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
    // Values read from openweathermap.org
    G_TEMP      = "TEMP"    // Current temperature
)
