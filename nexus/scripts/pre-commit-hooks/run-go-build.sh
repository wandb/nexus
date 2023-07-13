#!/usr/bin/env bash
# From: https://github.com/dnephin/pre-commit-golang/blob/master/run-go-build.sh


cd nexus
FILES=$(go list ./...  | grep -v /vendor/)
COMMIT=$(git rev-parse HEAD)
exec go build -ldflags "-X main.commit=$COMMIT" $FILES
