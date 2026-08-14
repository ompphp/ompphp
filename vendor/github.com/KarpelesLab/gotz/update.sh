#!/bin/bash
# This script rebuilds the timezone data files using files
# downloaded from the IANA distribution.
#
# Usage:
#   ./update.sh           # download, compile, and package tzdata
#   ./update.sh -work     # same but keep the work directory

# Versions to use.
CODE=2026a
DATA=2026a

set -e

cd "$(dirname "$0")"
rm -rf work
mkdir work
go build -o work/mkzip mkzip.go
cd work
mkdir zoneinfo

curl -sS -L -O "https://data.iana.org/time-zones/releases/tzcode${CODE}.tar.gz"
curl -sS -L -O "https://data.iana.org/time-zones/releases/tzdata${DATA}.tar.gz"
tar xzf "tzcode${CODE}.tar.gz"
tar xzf "tzdata${DATA}.tar.gz"

# Compile timezone data using zic.
# PACKRATDATA=backzone includes historical timezone data.
# PACKRATLIST=zone.tab includes all zones.
if ! make CFLAGS=-DSTD_INSPIRED AWK=awk TZDIR=zoneinfo PACKRATDATA=backzone PACKRATLIST=zone.tab posix_only >make.out 2>&1; then
	cat make.out
	exit 2
fi

# Copy metadata files into the zoneinfo directory for inclusion in the zip.
cp zone1970.tab iso3166.tab zoneinfo/

# Package compiled TZif files into a zip.
cd zoneinfo
../mkzip ../../zoneinfo.zip
cd ../..

echo "Updated zoneinfo.zip for tzcode${CODE}/tzdata${DATA}"

if [ "$1" = "-work" ]; then
	echo "Left workspace behind in work/."
else
	rm -rf work
fi
