package core

import (
    "flag"
)


var StartHour = flag.Int("starthour", 6, "Start hour for PV (e.g 6)")
var EndHour = flag.Int("endhour", 20, "End hour for PV (e.g 19)")

const (
    A_IN_TOTAL = "IN"
    A_OUT_TOTAL = "OUT"
    A_GEN_TOTAL = "GEN-T"
    A_GEN_DAILY = "GEN-D"
    A_IMPORT = "IMP"
    A_EXPORT = "EXP"
    G_GEN_P = "GEN-P"
    G_TP = "TP"
    G_VOLTS = "VOLTS"
)
