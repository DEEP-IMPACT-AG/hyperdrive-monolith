#!/usr/bin/env bash

set -e

rm -fr ./build
mkdir build

GOOS=linux go build -o build/main

aws cloudformation package \
    --profile deepimpact-dev \
    --template-file template.yml \
    --s3-bucket lambdacfartifacts-artifactbucket-10yx1c4johw49 \
    --s3-prefix lambda \
    --output-template-file packaged-template.yml