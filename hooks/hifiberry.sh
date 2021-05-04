#!/bin/bash

_() {
set -euo pipefail

case $1 in
    install)
        ;;

    kernel-command-line)
        echo dtoverlay=hifiberry-dac;;

    *)
        exit 1;;
esac
}

_ "$@"
