#!/bin/sh -e
{
set -x
unpaper "$1" "$1".pnm
rm "$1"
gm convert "$1".pnm "$1".png
rm "$1".pnm
} 2>&1 | tee -a /var/log/scn-sane-pp/scn-sane-pp.log
