#!/bin/bash

for file in $(find . -path .git -prune -o -name '*.go' -print); do go fmt $file; done
