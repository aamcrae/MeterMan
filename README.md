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

The core configuration allows a checkpoint file to be defined so that
state data is preserved across restarts.
The core configuration is:

```yaml
db:
  checkpoint: <checkpoint file>
  update: <interval for writing checkpoint file in seconds>
  freshness: <duration before data is considered stale>
  daylight: [<start hour>, <end hour>]
```

The default ```update``` interval is 60 seconds.
The ```freshness``` parameter (in minutes) defines how long data is not updated before
it is considered stale i.e not included in exports.  The default is 10 minutes.
The ```daylight``` parameters indicate the begin and end time (as hours) for the limit of daylight hours. The default is ```[5, 20]```.

The configuration for each the features are documented in:

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

A main thread is used for most processing, and only this thread can access the internal database to avoid
any concurrency issues.
Modules that provide data to the database do so by sending updates via a channel. These modules should run in separate
goroutines, and not as part of the main thread (otherwise there may be potential for deadlock with updates being sent to the channel
without being read). To access the database elements, modules can request functions to be executed from the main thread.
Timer callbacks can be registered, and these are aligned to the wall time with an optional offset e.g if a callback is requested
every minute with a -5 second offset, this is
invoked on 5 seconds before the minute (i.e seconds = 55). No I/O or extended processing should be performed in these callbacks (typically
any I/O is dispatched via a separate goroutine).

## Disclaimer

This is not an officially supported Google product.
