#!/bin/sh
set -eu
: "${OSS_BUCKET:?请设置 OSS_BUCKET}"
DATA_DIR="${EATBY_DATA_DIR:-/opt/eatby/production-data}"
OSSUTIL_BIN="${OSSUTIL_BIN:-ossutil}"
latest="$(find "$DATA_DIR/backups" -type f -name 'eatby-*.db' -print | sort | tail -n 1)"
if [ -z "$latest" ]; then
  echo "没有找到 EatBy 数据库备份" >&2
  exit 1
fi
"$OSSUTIL_BIN" cp -f "$latest" "oss://${OSS_BUCKET}/eatby/$(basename "$latest")"
