# MeterMan CSV data storage

The MeterMan CSV feature allows 5 minute interval data to be written to
daily CSV files for long term archival.

This is configured in the MeterMan YAML configuration file as:

```yaml
#
# CSV configuration
#
csv:
  base: <base directory>
  interval: <update interval in minutes>
```

The default update interval is 5 minutes

The base directory (e.g ```/var/lib/MeterMan/csv```) is used to store files in the format:
```
<basedirectory>/YYYY/MM/YYYY-MM-DD
```

With a base directory of ```/var/lib/MeterMan/csv```, an example file name is ```/var/lib/MeterMan/csv/2022/03/2022-03-14```

A new file is created each day.

The first line in the file is a commented title with column names.
An example:

```
#date,time,GEN-P,VOLTS,TEMP,IN-P,OUT-P,D-GEN-P,IMP,IMP-DAILY,EXP,EXP-DAILY,GEN-T,GEN-T-DAILY,GEN-D,GEN-D-DAILY,IN,IN-DAILY,OUT,OUT-DAILY
2024-05-14,00:00,,243.72,13.52,0.7,0,0,10984.44,0,19178.43,0,93382.9,0,23.86,0,10984.44,0,19178.43,0
2024-05-14,00:05,,243.21,13.48,0.57,0,0,10984.48,0.04,19178.43,0,93382.9,0,0,0,10984.48,0.04,19178.43,0
2024-05-14,00:10,,241.66,13.48,0.42,0,0,10984.52,0.08,19178.43,0,93382.9,0,0,0,10984.52,0.08,19178.43,0
```

Empty values indicate that the data is not fresh or available.
Some values are duplicated since they may come from different sources (e.g IMP and IN)

The columns are:

| Name | Unit | Description |
| ---- | ---- | -------- |
| date  | | The date as YYYY-MM-DD |
| time  | | The time as HH:MM (24 hour notation) |
| GEN-P | Kw | Solar PV power output  |
| VOLTS | V | Measured AC voltage |
| TEMP | degrees C | Temperature |
| IN-P | Kw | Power being imported from the grid |
| OUT-P | Kw | Power being exported to the grid |
| D-GEN-P | Kw | Calculated solar PV output (derived from PV running total) |
| IMP | KwH | Lifetime total of imported energy from grid |
| IMP-DAILY | KwH | Daily total imported energy from grid |
| EXP | KwH | Lifetime total of exported energy to grid |
| EXP-DAILY | KwH | Daily total exported energy to grid |
| GEN-T | KwH | Lifetime total of solar PV generated |
| GEN-T-DAILY | KwH | Daily total of solar PV generated |
| GEN-D | KwH | Derived total of solar PV generated |
| GEN-D-DAILY | KwH | Derived daily total of solar PV generated |
| IN | KwH | Lifetime total of imported energy from grid |
| IN-DAILY | KwH | Daily total of imported energy from grid |
| OUT | KwH | Lifetime total of exported energy to grid |
| OUT-DAILY | KwH | Daily total exported energy to grid |
