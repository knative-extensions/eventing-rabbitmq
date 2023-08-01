#!/usr/bin/env bash
for image in `docker images|grep kind.local | awk '{ print $1 }'`; do kind load docker-image $image; done
