# MeterMan Home Assistant upload

## MeterMan Configuration

MeterMan allows upload of data via REST to Home Assistant.
This is configured in the MeterMan YAML configuration file as:

```yaml
#
# Home Assistant integration
#
hassi:
  url: http://my-home-assistant.com:8123/api/states/sensor.meterman
  apikey: <long term api key>
  update: <Seconds between updates>
```

The default update interval is 120 seconds.

## Home Assistant integration

The template platform can be used to wrap the data so that it can be stored and graphed.
For example, in configuration.yaml:

```yaml
...
template: !include meterman.yaml
...
```

And meterman.yaml:

```yaml
# MeterMan sensor attributes
  - sensor:
    - name: "Export Daily"
      device_class: 'energy'
      state_class: "total"
      state: "{{ state_attr('sensor.meterman', 'export') | round(2) }}"
      unit_of_measurement: 'kWh'
    - name: "Import Daily"
      device_class: 'energy'
      state_class: "total"
      state: "{{ state_attr('sensor.meterman', 'import') | round(2) }}"
      unit_of_measurement: 'kWh'
    - name: "Solar Daily"
      device_class: 'energy'
      state_class: "total"
      state: "{{ state_attr('sensor.meterman', 'gen_daily') | round(2) }}"
      unit_of_measurement: 'kWh'
    - name: "Solar generation"
      device_class: 'power'
      state_class: "measurement"
      state: "{{ (state_attr('sensor.meterman', 'gen_power') | float * 1000) | round(0) }}"
      unit_of_measurement: 'W'
    - name: "Export power"
      device_class: 'power'
      state_class: "measurement"
      state: "{% if state_attr('sensor.meterman', 'meter_power') >= 0 %}0{% else %}{{ (state_attr('sensor.meterman', 'meter_power') | float * -1000) | round(0) }}{% endif %}"
      unit_of_measurement: 'W'
    - name: "Import power"
      device_class: 'power'
      state_class: "measurement"
      state: "{% if state_attr('sensor.meterman', 'meter_power') < 0 %}0{% else %}{{ (state_attr('sensor.meterman', 'meter_power') | float * 1000) | round(0) }}{% endif %}"
      unit_of_measurement: 'W'
    - name: "Voltage"
      device_class: 'voltage'
      state_class: "measurement"
      state: "{{ state_attr('sensor.meterman', 'volts') | round(1) }}"
      unit_of_measurement: 'V'
    - name: "Daily consumption"
      device_class: 'energy'
      state_class: "total"
      state: "{{ (float(states('sensor.import_daily')) + float(states('sensor.solar_daily')) - float(states('sensor.export_daily'))) | round(2) }}"
      unit_of_measurement: 'kWh'
    - name: "Consumption"
      device_class: 'power'
      state_class: "measurement"
      state: "{{ (float(states('sensor.import_power')) + float(states('sensor.solar_generation')) - float(states('sensor.export_power'))) | round(0) }}"
      unit_of_measurement: 'W'
    - name: "Current power"
      device_class: 'power'
      state_class: "measurement"
      state: "{{ float(states('sensor.export_power')) - float(states('sensor.import_power')) | round(0) }}"
      unit_of_measurement: 'W'
```

The 'Import Daily', 'Export Daily' and 'Solar Daily' can be used as inputs to the Home Assistant Energy dashboard.
