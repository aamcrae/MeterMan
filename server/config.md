# MeterMan API

MeterMan may expose a HTTP server that provides a JSON API and a basic
status page to be accessed.

This server is configured in the YAML configuration file as:

```yaml
#
# API configuration
#
api:
  port: <server port number>
```

The default ```port``` number is 8080.
The server provides multiple endpoints; accessing ```/status```
displays some basic status information. Accessing ```/api``` provides a
JSON encoded structure of most of the core data values such as power, energy (total
and daily values) etc.
