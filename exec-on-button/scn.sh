#!/bin/sh -e
FN=/tmp/scan-$$
set -x
#scanadf -d 'fujitsu:ScanSnap S510:7301' -o $FN-%04d.tiff
scanadf -o $FN-%04d.tiff
for nm in $FN-*.tiff; do
	gm convert $nm $(dirname $nm)/$(basename $nm .tiff).jpg
	rm $nm
done
/home/pi/bin/camput file --permanode --tag scanner $FN*.jpg
rm -rf $FN*.jpg
