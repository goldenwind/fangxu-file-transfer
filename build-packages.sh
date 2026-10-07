#!/bin/sh
# Build the native desktop installer on macOS or Linux. On Windows use npm.
set -eu
transfer_source_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$transfer_source_dir"
command -v node >/dev/null
command -v cargo >/dev/null
command -v go >/dev/null
if [ ! -d node_modules ]; then npm ci; fi
npm run desktop:build -- "$@"
