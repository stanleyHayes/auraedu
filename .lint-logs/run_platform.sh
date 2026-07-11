#!/bin/bash
cd /Users/shayford/Desktop/Dev/Projects/auraedu/platform
GOTOOLCHAIN=go1.26.5 GOWORK=off /Users/shayford/go/bin/golangci-lint run --concurrency 1 ./... > /Users/shayford/Desktop/Dev/Projects/auraedu/.lint-logs/platform.log 2>&1
echo EXIT:$? >> /Users/shayford/Desktop/Dev/Projects/auraedu/.lint-logs/platform.log
