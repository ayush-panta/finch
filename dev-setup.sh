#!/bin/bash
set -e

git submodule update --init --recursive
unset GOSUMDB
make clean
make
./_output/bin/finch vm init