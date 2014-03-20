#!/bin/bash
export GOPATH=/home/3pkg:$PWD
export GOBIN=$PWD/bin
go install main
cd bin
./main
