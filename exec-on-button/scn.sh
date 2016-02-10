#!/bin/sh -e
FN=/tmp/scan-$$
#scanadf -d 'fujitsu:ScanSnap S510:7301' -o $FN-%04d.tiff
scanadf -o $FN-%04d.tiff
/home/pi/bin/camput file --permanode --tag scanner $FN*.tiff
rm -rf $FN*.tiff
