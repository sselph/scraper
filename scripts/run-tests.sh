#!/bin/bash
set -ev
if [ "${GIMME_OS}" = "linux" ] && [ "${GIMME_ARCH}" = "amd64" ]; then
	go test -v -tags noasm ./ds
	go test -v -tags noasm ./rom/hash
	go test -v -tags noasm ./ss
fi
