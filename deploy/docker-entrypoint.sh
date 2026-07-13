#!/bin/sh
set -eu
mkdir -p /data /data/backups
chown -R pantry:pantry /data
exec su-exec pantry pantry -data /data
