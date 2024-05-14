# MeterMan PVoutput

MeterMan can upload data to [PVOutput](http://pvoutput.org), an open service that
allows tracking of solar PV generation.

PVOutput is configured in the YAML configuration file as:

```yaml
#
# pvoutput configuration
#
pvoutput:
  apikey: <apikey from pvoutput.org>
  systemid: <systemid from pvoutput.org>
  pvurl: <URL API endpoint to use>
  interval: <upload interval in minutes>
  trace: <true/false>
```

The default ```interval``` value is 5. If ```trace``` is set to ```true```, the upload
transactions are logged.

The default ```pvurl``` is ```https://pvoutput.org/service/r2/addstatus.jsp```.

MeterMan will attempt to upload the solar PV daily generation, the current PV power,
the daily energy consumption, the current power consumption, the voltage and the temperature.
If any of the values are not fresh or valid, they are not uploaded.
