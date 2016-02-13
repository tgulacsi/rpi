#!/bin/sh -e
nice -n 10 unpaper "$1" "$1"-unpapered
rm "$1"
gm convert "$1"-unpapered "$1".png
rm "$1"-unpapered
camput file -permanode -tag scan "$1".png
rm "$1".png
exit 1
