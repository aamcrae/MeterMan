# MeterMan IAMMETER

A [IAMMETER](https://www.iammeter.com/products/single-phase-meter) Energy Meter may be
monitored. Only the single phase meter is supported at this stage.

The IAMMETER meter is configured in the YAML configuration file as:

```yaml
#
# iammeter configuration
#
iammeter:
  meter: <url to retrieve data>
  poll: <polling interval in seconds>
  offset: <polling offset in seconds>
```

The meter URL is usually of the form ```http://user:password@<meter>/monitorjson```
where ```<meter>``` is the host name or IP address of the meter, and the default user and password
is ```admin:admin```.
The default polling interval is 30 seconds, and default offset is -5 seconds

The values read from the meter are the voltage, current, power (import or export)
and total export and import energy.
