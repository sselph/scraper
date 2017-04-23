#!/bin/bash
set -ev
if [ "${GIMME_OS}" = "linux" ] && [ "${GIMME_ARCH}" = "amd64" ]; then
	go test -v ./ds
	go test -v ./rom/hash
	go test -v ./ss
fi
