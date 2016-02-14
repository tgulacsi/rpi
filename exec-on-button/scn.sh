#!/bin/sh -e
FN=/tmp/scan-$(date '+%Y%m%d_%H%M%S')
set -x
#	-S /home/pi/bin/scn-sane-pp.sh \
scanadf \
	--source 'ADF Duplex' \
	--mode Gray \
	--resolution 300 \
	-o $FN-%04d.pnm
#/home/pi/bin/camput file --permanode --tag scanner $FN*.png
zip -2 $FN.zip $FN*.pnm
rm -f $FN*.pnm

ENVFN="$(dirname $0)/scn-sane-pp.sh.env"
if [ -e $ENVFN ]; then
	# contains AMQP_USER and AMQP_PASSWORD
	. $ENVFN
fi
/home/pi/bin/amqpc pub --no-compress @$FN.zip
rm -rf $FN*
