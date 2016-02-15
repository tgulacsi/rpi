#!/bin/sh -e
pnm2png () {
	fn="$1"
	D=$(cd $(dirname $fn); pwd)
	B=$(basename $fn .pnm)
	ofn="$D/$B-bw.png"
	optimize2bw -n -i "$fn" -o "$ofn"
	rm "$fn"
	fn="$ofn"
	if empty-page -i "$fn" 2>&1 | grep '^empty'; then
		rm "$fn"
		echo 'page is empty' >&2
	else
		ofn="$D/$B-bw-unpapered.png"
		nice -n 10 unpaper "$fn" "$ofn"
		rm "$fn"
		fn="$ofn"

		$HOME/bin/camput file -permanode -tag scan "$fn"
		echo "PNG=$fn"
	fi
}

echo "### $@ ###"
date
set -x
fn="$1"
. $HOME/.profile
MIME=$(file -i "$fn")
if echo "$MIME" | grep -q 'image/'; then
	echo "MIME=$MIME"
elif echo "$MIME" | grep -q 'application/zip'; then
	rm -rf "$fn.d"
	mkdir -p "$fn.d"
	cd "$fn.d"
	unzip -jo "$fn" 
	#rm "$fn"
	pwd; ls -lA
	ls | while read nm; do
		pnm2png "$nm"
	done
	pwd; ls -lA
	gm convert *.png "$fn.pdf"
	gdrive upload -f "$fn.pdf"
	cd $(dirname $(dirname "$fn.d"))
	rm -rf "$fn.d"
	exit 0
else
	echo "MIME=$MIME, skipping!" >&2
	exit 0
fi

pnm2png "$fn"
rm "$fn*"
