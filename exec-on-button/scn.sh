#!/bin/sh -e
FN=/tmp/scan-$$
set -x
scanadf \
	-S /home/pi/bin/scn-sane-pp.sh \
	--source 'ADF Duplex' \
	--mode Grayscale \
	--resolution 300 --y-resolution 300 \
	-o $FN-%04d.pnm
#/home/pi/bin/camput file --permanode --tag scanner $FN*.png
rm -rf $FN*.png
