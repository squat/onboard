# Onboard

`onboard` is a tool designed to help you connect single board computers (SBCs), such as Raspberry Pis, to your fleet of IoT devices.
It provides a configurable wizard via a webapp to input all of the information needed to connect the device to a wireless network and to start the services you need.

[![Go Report Card](https://goreportcard.com/badge/github.com/squat/onboard)](https://goreportcard.com/report/github.com/squat/onboard)

## Overview


## Getting Started

First, flash the `onboard` filesystem onto an SD card using the [`install.sh`](https://github.com/squat/onboard/blob/master/install.sh) script, or with the following snippet:

```shell
curl -sfL https://raw.githubusercontent.com/squat/onboard/master/install.sh | sh -
```

This will install a complete Linux distribution onto the SD card.

_Note_: the script is currently compatible with the following IoT devices:
* Raspberry PI 4;
* Raspberry PI 3;
* Raspberry PI Zero W; and
* Banana PI M2 Zero.

Next, insert the SD card into the IoT device and power it on.
The device will take ~10 seconds to boot, after which `onboard` will start a wireless access point with the name `onboard`.
To access the `onboard` webapp, connect your computer to the `onboard` wireless network and point a browser to [http://onboard.local/](http://onboard.local/).
The `onboard` webapp will ask for some information so that it can connect to your local wireless network and to any services specified in its configuration file.

## Usage

[embedmd]:# (tmp/help.txt)
```txt
Usage of bin/amd64/onboard:
  -c, --config stringArray            The path to the configuration file for Onboard. Can be specified multiple times to concatenate mutiple configuration files. Can be a glob, e.g. /path/to/configs/*.yaml. Files are processed in lexicographic order.
      --debug.name string             A name to add as a prefix to log lines. (default "onboard")
      --id string                     The ID for this device.
      --ip-address string             The IP address of the device running this process. (default "10.0.0.1")
      --log.format string             The log format to use. Options: 'logfmt', 'json'. (default "logfmt")
      --log.level string              The log filtering level. Options: 'error', 'warn', 'info', 'debug'. (default "info")
      --web.healthchecks.url string   The URL against which to run healthchecks. (default "http://localhost:8080")
      --web.internal.listen string    The address on which the internal server listens. (default ":8081")
      --web.listen string             The address on which the public server listens. (default ":8080")
      --wlan-interface string         The name of the WLAN interface to configure. (default "wlan0")
```
