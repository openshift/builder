#!/bin/bash

bad_files=$(go run ./vendor/github.com/openshift-eng/openshift-goimports -m github.com/openshift/builder -l)
if [[ -n "${bad_files}" ]]; then
        echo "!!! openshift-goimports needs to be run on the following files:"
        echo "${bad_files}"
        echo "Try running 'make imports'"
        exit 1
fi
