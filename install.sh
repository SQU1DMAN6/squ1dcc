#!/usr/bin/env bash

BLUE='\033[0;34m'
NC='\033[0m'

echo "Installing SQU1D++ SQU1DLang compiler..."
go build -o squ1d++ .

sudo cp squ1d++ /usr/local/bin/
sudo mkdir /usr/share/squ1d++/
sudo cp squ1d++ /usr/share/squ1d++/

sudo rm -rf /tmp/fsdl

echo -e "SQU1D++ compiler installed successfully. Run ${BLUE}squ1d++${NC} as terminal command."
