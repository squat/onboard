# Onboard

`onboard` is a tool designed to help you connect single board computers (SBCs), such as Raspberry Pis, to your fleet of IoT devices.
It provides a configurable wizard via a webapp to collect all of the information needed to configure the device, such as connecting to a wireless network and starting the services you need.

[![Go Report Card](https://goreportcard.com/badge/github.com/squat/onboard)](https://goreportcard.com/report/github.com/squat/onboard)

## Overview

`onboard` provides two main pieces of functionality to connect your IoT devices to your fleet:
1. a programmable and extensible service for configuring the IoT device; this service:
    1. runs a webapp that guides the user through a wizard in order to provide any necessary inputs;
    2. executes the requested operations on the host, such as provisioning files and running systemd units;
    3. verifies the proper operation of the host by running a series of configured checks; and
    4. surfaces the status and progress of the configuration process to the user through the webapp.

2. a minimalist base OS for the IoT device built on [Arch Linux Arm](https://archlinuxarm.org/) that can be installed using the [`install.sh`](https://github.com/squat/onboard/blob/master/install.sh) script; this base OS:
    1. starts a wireless access point with an open wireless network named `onboard`;
    2. runs the `onboard` configuration wizard webapp at [http://onboard.local/](http://onboard.local/);
    3. configures the IoT device using the collected inputs to connect to a desired wireless network;
    4. runs any other actions specified via the `onboard` configuration files, such as starting additional systemd services;
    5. disables the wireless access point once the network connection has been successfully established; and
    5. periodically checks if the network is inaccessible, in which case it re-enables the wireless access point so that the device can be reconfigured.

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
