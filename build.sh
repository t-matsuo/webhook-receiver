#!/bin/bash

SCRIPT_DIR=$(cd $(dirname $0); pwd)
[ -d "$SCRIPT_DIR/_build/" ] || mkdir $SCRIPT_DIR/_build

OS_ARCH="darwin_amd64 darwin_arm64 freebsd_386 freebsd_amd64 freebsd_arm linux_386 linux_amd64 linux_arm netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64  plan9_386 plan9_amd64 windows_386 windows_amd64"

if [ "$GO_OS_ARCH" != "" ]; then
    OS=`echo $GO_OS_ARCH | cut -d "_" -f 1`
    ARCH=`echo $GO_OS_ARCH | cut -d "_" -f 2`
    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build "-ldflags=-s -w -buildid=" -trimpath -o $SCRIPT_DIR/_build/webhook_$GO_OS_ARCH webhook.go
    exit $?
fi

for i in $OS_ARCH; do
    OS=`echo $i | cut -d "_" -f 1`
    ARCH=`echo $i | cut -d "_" -f 2`
    echo $OS $ARCH
    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build "-ldflags=-s -w -buildid=" -trimpath -o $SCRIPT_DIR/_build/webhook_$i webhook.go
done
