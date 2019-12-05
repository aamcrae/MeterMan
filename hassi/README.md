# MeterMan Home Assistant upload

MeterMan allows upload of data via REST to Home Assistant.
This is configured in the MeterMan configuration as:

'''
#
# Home Assistant integration
#
[hassi]
url=http://my-home-assistant.com:8123/api/states/sensor.meterman
apikey=<long term api key>
'''

The template platform can be used to wrap the data so that it can be stored and graphed:

'''yaml
sensor:
  - platform: template
    sensors:
      consumption:
        friendly_name: "Daily consumption"
        unit_of_measurement: 'kWh'
        value_template: "{{ float(states('sensor.import_total')) + float(states('sensor.pv_daily')) - float(states('sensor.export_total')) }}"
      export_total:
        friendly_name: "Export Daily"
        unit_of_measurement: 'kWh'
        value_template: "{{ state_attr('sensor.meterman', 'export') }}"
      import_total:
        friendly_name: "Import Daily"
        unit_of_measurement: 'kWh'
        value_template: "{{ state_attr('sensor.meterman', 'import') }}"
      pv_daily:
        friendly_name: "Solar Daily"
        unit_of_measurement: 'kWh'
        value_template: "{{ state_attr('sensor.meterman', 'gen_daily') }}"
      pv_power:
        friendly_name: "Solar generation"
        unit_of_measurement: 'Watt'
        value_template: "{{ state_attr('sensor.meterman', 'gen_power') | float * 1000 }}"
      export:
        friendly_name: "Export power"
        unit_of_measurement: 'Watt'
        value_template: "{% if state_attr('sensor.meterman', 'meter_power') >= 0 %}0{% else %}{{ state_attr('sensor.meterman', 'meter_power') | float * -1000 }}{% endif %}"
      import:
        friendly_name: "Import power"
        unit_of_measurement: 'Watt'
        value_template: "{% if state_attr('sensor.meterman', 'meter_power') < 0 %}0{% else %}{{ state_attr('sensor.meterman', 'meter_power') | float * 1000 }}{% endif %}"
      consumption_power:
        friendly_name: "Current consumption"
        unit_of_measurement: 'Watt'
        value_template: "{{ float(states('sensor.import')) + float(states('sensor.pv_power')) - float(states('sensor.export')) }}"
      voltage:
        friendly_name: "Voltage"
        unit_of_measurement: 'Volts'
        value_template: "{{ state_attr('sensor.meterman', 'volts') }}"
'''
