# MeterMan

MeterMan is a utility to monitor electricity consumption and
Solar Photovoltaic (PV) output, and send the results to various collectors.

The features include:
* Using the [LCD](http://github.com/aamcrae/lcd) library to read electrical meter LCD screens via a webcam.
* Monitoring [SMA](http://sma.de) Solar inverters
* Retrieving the current temperature via a weather provider.
* Uploading the data to [PVOutput](http://pvoutput.org)
* Uploading data to [Home Assistant](http://www.home-assistant.io).
* Saving 5 minute snapshots in CSV files.

An internal database allows input and output modules to be added independently, and multiple values to
be averaged, summed and managed.

MeterMan can be built as a docker image:

```
docker build --tag aamcrae/meterman:latest .
```

A [sample](docker/sample-docker-compose.yml) docker compose file can be to
deploy the image.

It is recommended that a MeterMan be run under its own uid (e.g 'meter:meter').

This is not an officially supported Google product.
