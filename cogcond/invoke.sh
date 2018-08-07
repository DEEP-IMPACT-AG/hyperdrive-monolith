#!/usr/bin/env bash

set -e

rm -fr ./build
mkdir build

GOOS=linux go build -o build/main

sam local invoke "Function" -e test/cog-event.json -n test/env.json --profile deepimpact-dev