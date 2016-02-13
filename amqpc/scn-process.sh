#!/bin/sh -e
set -x
fn="$1"
. $HOME/.profile
MIME=$(file -i "$fn")
if echo "$MIME" | grep -q 'image/'; then
	echo "MIME=$MIME"
else
	echo "MIME=$MIME, skipping!" >&2
	exit 0
fi
empty-page -i "$fn" | grep '^empty' && { rm "$fn"; exit 0; }

optimize2bw -n -i "$fn" -o "${fn}-bw.png"
rm "$fn"
fn="${fn}-bw.png"
nice -n 10 unpaper "$fn" "$fn"-unpapered
rm "$fn"
fn="${fn}-unpapered"
econvert -i "$fn" -o "$fn".png
rm "$fn"
fn="${fn}.png"

$HOME/bin/camput file -permanode -tag scan "$fn"
rm "$fn"
