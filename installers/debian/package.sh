#!/bin/bash

# Install packages dh_make, debhelper

# You will recieve two prompts. Enter 's' for the first and 'y' for the second.

set -ex

VERSION=$1
if [[ ! $VERSION ]]; then
	echo "no version specified"
	exit 1
fi
# Need revision too

ROOT_DIR=$(realpath "$(dirname "${BASH_SOURCE[0]}")")
cd ${ROOT_DIR}

ECOSERVICE_DIR=`cd ../../cmd/ecoservice; pwd`
ECOGUI_DIR=`cd ../../cmd/ecogui; pwd`

BUILD_DIR=${ROOT_DIR}/eco-${VERSION}
mkdir -p ${BUILD_DIR}
cd ${BUILD_DIR}

dh_make --native

BIN_DIR=${BUILD_DIR}/usr/local/bin
mkdir -p ${BIN_DIR}

cd ${ECOSERVICE_DIR}
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ${BIN_DIR}/ecoservice

cd ${ECOGUI_DIR}
env CGO_ENABLED=1 go build -a -ldflags="-s -w" -o ${BIN_DIR}/ecogui

cd ${ROOT_DIR}
DESKTOP_FILE_DIR=${BUILD_DIR}/usr/local/share/applications
mkdir -p ${DESKTOP_FILE_DIR}
DESKTOP_FILE=${DESKTOP_FILE_DIR}/ecogui.desktop
cp ecogui.desktop ${DESKTOP_FILE}
echo -e "VERSION=${VERSION}\n" >> ${DESKTOP_FILE}

ICON_DIR=${BUILD_DIR}/usr/local/share/pixmaps
mkdir -p ${ICON_DIR}
cp ecogui.png ${ICON_DIR}/ecogui.png

APP_DIR=${BUILD_DIR}/opt/decred-eco
mkdir -p ${APP_DIR}
cp -r ${ECOGUI_DIR}/static ${APP_DIR}

cp eco.service ${BUILD_DIR}/debian
cd ${BUILD_DIR}
dh_installinit
dpkg-buildpackage -us -uc



