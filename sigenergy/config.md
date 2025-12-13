# MeterMan SigEnery battery

Monitoring for SigEnergy battery.

The SigEnergy battery is configured in the YAML configuration file as:

```yaml
#
# SigEnery configuration
#
sigenergy:
  addr: <battery-name:udp-port>
  unit: <modbus-unit-id>
  poll: <poll-seconds>
  offset: <poll offset>
  timeout: <timeout-seconds>
  trace: <true/false>
```

The battery name may be a host name or an IP address.

The default poll interval is 60 seconds, and default offset is -5 seconds.
The timeout default is 10 seconds. Enabling ```trace``` will turn
on logging of packet connections to the battery and dumping of packets.
