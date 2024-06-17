# MeterMan SMA Inverter

MeterMan can monitor one or more SMA Inverters, retrieving
power and energy values and feeding these into the database for
external output.

The SMA inverters are configured in the YAML configuration file as:

```yaml
#
# SMA configuration
#
sma:
  - addr: <inverter-name:udp-port>
    password: <password>
    poll: <poll-seconds>
    offset: <poll offset>
    timeout: <timeout-seconds>
    volts: <true/false>
    trace: <true/false>
    dump: <true/false>
  ...
```

The inverter name may be a host name or an IP address.

The default poll interval is 60 seconds, and default offset is -5 seconds.
The timeout default is 10 seconds. Enabling ```trace``` and ```dump``` will turn
on logging of packet connections to the inverter and dumping of packets.
Enabling ```volts``` will monitor and save the inverter voltage readings.
