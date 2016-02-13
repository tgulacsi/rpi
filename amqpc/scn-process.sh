#!/bin/sh -e
MIME=$(file -i "$1")
if echo "$MIME" | grep -q 'image/'; then
	echo "MIME=$MIME"
else
	echo "MIME=$MIME, skipping!" >&2
	exit 0
fi
nice -n 10 unpaper "$1" "$1"-unpapered
rm "$1"
gm convert "$1"-unpapered "$1".png
rm "$1"-unpapered
$HOME/bin/camput file -permanode -tag scan "$1".png
rm "$1".png
