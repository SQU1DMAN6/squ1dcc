#!/usr/bin/env bash

BLUE='\033[0;34m'
NC='\033[0m'

echo "Installing SQU1D++ SQU1DLang compiler..."
unzip -oqq squ1dcc.fsdl
go build -o squ1d++ .

sudo cp squ1d++ /usr/local/bin/
sudo mkdir -p /usr/share/squ1d++/
sudo cp squ1d++ /usr/share/squ1d++/

sudo rm -rf /tmp/fsdl

echo SQU1D++ has been installed to $(which squ1d++).

echo -e "SQU1D++ compiler installed successfully. Run ${BLUE}squ1d++${NC} as terminal command."
