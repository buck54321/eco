#!/bin/bash

set -ex

# Make sure a version is supplied.
VERSION=$1
if [[ ! $VERSION ]]; then
	echo "no version specified"
	exit 1
fi

# Just in case someone is running this from another directory, but the actual
# script directory.
ROOT_DIR=$(realpath "$(dirname "${BASH_SOURCE[0]}")")
cd ${ROOT_DIR}

# And we know the other cmd paths relative to the script.
ECOSERVICE_DIR=`cd ../ecoservice; pwd`
ECOGUI_DIR=`cd ../ecogui; pwd`

# Create a temporary build directory.
BUILD_DIR=${ROOT_DIR}/include
# Clean up any old build directories. Shouldn't be any unless a previous run
# failed.
rm -r -f ${BUILD_DIR}
# Copy our linux template.
cp -r ${ROOT_DIR}/include-linux ${BUILD_DIR}

# Insert the version.
echo ${VERSION} > ${BUILD_DIR}/version
# Add the version to the desktop file.
echo -e "VERSION=${VERSION}\n" >> ${BUILD_DIR}/ecogui.desktop

# Build ecoservice
cd ${ECOSERVICE_DIR}
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ${BUILD_DIR}/ecoservice

# Build ecogui
cd ${ECOGUI_DIR}
env CGO_ENABLED=1 go build -a -ldflags="-s -w" -o ${BUILD_DIR}/ecogui

# Copy ecogui static files.
cp -r ${ECOGUI_DIR}/static ${BUILD_DIR}

# Build archive.go first, then our executable.
cd ${ROOT_DIR}
go run binvars/main.go
env CGO_ENABLED=1 go build -a -ldflags="-s -w" -o ecoinstaller_${VERSION}

# Clean up and replace the huge archive file with a smaller dummy that doesn't
# break my IDE with auto-tests.
rm -r ${BUILD_DIR}
rm archive.go
cp dummyarchive.nogo archive.go
