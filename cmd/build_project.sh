#!/bin/bash

NAME="alfred"
VERSION=$(git tag | tail -n 1)

DIR=${NAME}-${VERSION}
mkdir $DIR

GOOS=linux go build --tags netgo --ldflags '-extldflags "-lm -lstdc++ -static"' -o ${DIR}/${NAME} main.go


