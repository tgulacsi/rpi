#!/bin/sh -e
LOGFN=/var/log/scn-sane-pp/scn-sane-pp.log
mkdir -p $(dirname $LOGFN)
ENVFN="$0.env"
if [ -e $ENVFN ]; then
	# contains AMQP_USER and AMQP_PASSWORD
	. $ENVFN
fi
/home/pi/bin/amqpc pub @"$1" 2>&1 | tee -a $LOGFN
