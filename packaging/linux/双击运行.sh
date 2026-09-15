#!/bin/sh

transfer_linux_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec "$transfer_linux_dir/fangxu-file-transfer"
