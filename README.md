# MeterMan

MeterMan is a utility to monitor electricity consumption and
Solar Photovoltaic (PV) output, and send the results to various collectors.

The features include:
* Using the [LCD](http://github.com/aamcrae/lcd) library to read electrical meter LCD screens via a webcam.
* Monitoring using an [IAMMeter](https://www.iammeter.com/products/single-phase-meter) Energy Meter
* Monitoring [SMA](http://sma.de) Solar inverters
* Retrieving the current temperature via a weather provider.
* Uploading the data to [PVOutput](http://pvoutput.org)
* Uploading data to [Home Assistant](http://www.home-assistant.io).
* Saving 5 minute snapshots to CSV files.
* Access to data via a JSON API

## Configuration

MeterMan uses a [YAML](https://yaml.org/) based configuration file, as well as
various command line flags - running ```MeterMan --help``` will list the available flags.

The [example configuration file](example.conf) shows how each feature can be
individually enabled and configured. If a feature is required, it is enabled
by adding that feature's configuration to the YAML configuration file.

The configuration and flags for each the features are documented in:

* [SMA](sma/config.md) - Monitoring of [SMA](http://sma.de) Solar inverters.
* [IAMMeter](iammeter/config.md) - Monitoring of [IAMMeter](https://www.iammeter.com/products/single-phase-meter) Energy meters.
* [Weather](weather/config.md) - Retrieval of current temperature from a selectable weather service.
* [Meter](meter/config.md) - Reading of LCD displays on electricity meters via a webcam.
* [CSV](csv/config.md) - 5 minute snapshots of data to daily [Comma Separated Values](https://en.wikipedia.org/wiki/Comma-separated_values) files.
* [Home Assistant](hassi/config.md) - Uploading of data to a [Home Assistant](http://www.home-assistant.io) instance.
* [PvOutput](pv/config.md) - Uploading 5 minute interval data to [PVOutput](http://pvoutput.org).
* [API](server/config.md) - JSON API for export of monitored data.

## Building and Running

MeterMan can be built as a docker image:

```
docker build --tag aamcrae/meterman:latest -f docker/Dockerfile .
```

A [sample](docker/sample-docker-compose.yml) docker compose file can be to
deploy the image.

It is recommended that a MeterMan be run under its own uid (e.g 'meter:meter').

## Internals

MeterMan is written in [Go](https://go.dev/) and uses a [YAML](https://yaml.org/)
based configuration file.

An internal database allows input and output modules to be added independently, and multiple values to
be averaged, summed and managed.

## Disclaimer

This is not an officially supported Google product.
