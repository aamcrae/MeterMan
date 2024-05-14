# MeterMan Temperature

A number of different weather services may be configured that allows
reading the current temperature. Depending on which weather service
is selected, different parameters may be required.

The weather service is configured in the YAML configuration file as:

```yaml
#
# temperature configuration
#
weather:
  tempservice: <bom,openweather,darksky>
  poll: <interval for poll in seconds>
# If 'bom' is selected
  bom: http://www.bom.gov.au/fwo/<bom URL>
# If 'openweather' is selected
  tempid: <location id>
  tempkey: <key>
# if 'darksky' is selected
  darkskykey: <key>
  darkskylat: <latitude>
  darkskylong: <longitude>
```

The ```poll``` parameter (default 120 seconds) configures the interval between polling the service.
Only the temperature is retrieved from the service.
