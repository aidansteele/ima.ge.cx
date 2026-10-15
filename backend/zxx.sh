#!/bin/sh
set -eu
export ZIG_SYSTEM_LINKER_HACK=1
export ZIG_LOCAL_CACHE_DIR="$HOME/.zigcache/"
zig c++ -target $ZTARGET "$@"
