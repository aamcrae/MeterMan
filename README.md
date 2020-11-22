# MeterMan

MeterMan is a utility to monitor electricity consumption and
Solar Photovoltaic (PV) output, and send the results to various collectors.

The features include:
* Using the [LCD](github.com/aamcrae/lcd) to read electrical meter LCD screens via a webcam.
* Monitoring [SMA](sma.de) Solar inverters
* Retrieving the current temperature via a weather provider.
* Uploading the data to [PVOutput](pvoutput.org)
* Uploading data to [Home Assistant](www.home-assistant.io).
* Saving 5 minute snapshots in CSV files.

An internal database allows input and output modules to be added independently, and multiple values to
be averaged, summed and managed.

This is not an officially supported Google product.
