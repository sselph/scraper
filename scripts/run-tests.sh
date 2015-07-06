#!/bin/bash
set -ev
if [ "${GIMME_OS}" = "linux" ]; then
	go test -v ./ds
	go test -v ./rom/hash
fi
