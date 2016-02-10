#!/bin/sh -e
unpaper "$1" "$1".pnm
rm "$1"
gm convert "$1".pnm "$1".png
rm "$1".pnm
