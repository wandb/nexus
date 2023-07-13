#!/usr/bin/env bash
#
# Script used to configure nexus debug env variable which informs
# wandb/wandb to use this nexus dev dir through the _WANDB_NEXUS_PATH
# environment variable
#
# Usage:
#   source scripts/dev-nexus.sh
#   source scripts/dev-nexus.sh --unset

set -e
ARG=$1
BASE=$(dirname $(dirname $(readlink -f $0)))

if [ "x$ARG" = "x--unset" ]; then
    echo "[INFO]: Clearing nexus dev dir."
    unset _WANDB_NEXUS_PATH
elif [ "x$ARG" = "x" ]; then
    echo "[INFO]: Setting nexus dev dir to ${BASE}."
    export _WANDB_NEXUS_PATH=${BASE}/scripts/run-nexus.sh
else
    echo "[ERROR]: Unhandled arg: $ARG." 2>&1
    exit 1
fi
