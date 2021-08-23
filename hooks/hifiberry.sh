#!/bin/bash

install() {
    exit 0
}

done-file() {
    exit 1
}

kernel-command-line() {
    echo dtoverlay=hifiberry-dac
}
