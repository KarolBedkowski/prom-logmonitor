#
# Makefile
#

# Path to raspberry pi mounted root (by sshfs)
RPIROOT=/home/k/mnt/pi/

# enable sdjournal
#GOTAGS=
GOTAGS=-tags 'sdjournal'

#
VERSION=1.1
REVISION=`git describe --always`
DATE=`date +%Y%m%d%H%M%S`
USER=`whoami`
BRANCH=`git branch | grep '^\*' | cut -d ' ' -f 2`
LDFLAGS="\
	-X github.com/prometheus/common/version.Version=$(VERSION) \
	-X github.com/prometheus/common/version.Revision='$(REVISION) \
	-X github.com/prometheus/common/version.BuildDate=$(DATE) \
	-X github.com/prometheus/common/version.BuildUser=$(USER) \
	-X github.com/prometheus/common/version.Branch=$(BRANCH)"
LDFLAGS_PI="-w -s \
	-X github.com/prometheus/common/version.Version=$(VERSION) \
	-X github.com/prometheus/common/version.Revision=$(REVISION) \
	-X github.com/prometheus/common/version.BuildDate=$(DATE) \
	-X github.com/prometheus/common/version.BuildUser=$(USER) \
	-X github.com/prometheus/common/version.Branch=$(BRANCH)"

logmonitor: build

.PHONY: build
build: 
	CGO_ENABLED=1 go build $(GOTAGS) -v -o logmonitor -ldflags $(LDFLAGS)

.PHONY: build_pi
build_pi: 
	GOGCCFLAGS="-fPIC -O4 -Ofast -pipe -march=native -mcpu=arm1176jzf-s -mfpu=vfp -mfloat-abi=hard -s" \
		GOARCH=arm GOARM=6 CGO_ENABLED=1 GOOS=linux \
		CC=arm-linux-gnueabi-gcc  \
		CXX=arm-linux-gnueabi-g++ \
		CGO_LDFLAGS="-L$(RPIROOT)/lib/arm-linux-gnueabihf -lsystemd \
			-Wl,-rpath-link=$(RPIROOT)/lib/arm-linux-gnueabihf \
			-Wl,-rpath-link=$(RPIROOT)/lib/ \
			-Wl,-rpath-link=$(RPIROOT)/usr/lib/ \
			-Wl,-rpath-link=$(RPIROOT)/usr/lib/arm-linux-gnueabihf " \
		go build  $(GOTAGS) -v -o logmonitor-arm -ldflags $(LDFLAGS_PI)

logmonitor-arm: build_pi

.PHONY: install_pi
install_pi: logmonitor-arm
	ssh pi "systemctl --user stop logmonitor"
	ssh pi "[ -f ~/prometheus/logmonitor-arm ] && mv -f ~/prometheus/logmonitor-arm ~/prometheus/logmonitor-arm.old"
	scp logmonitor-arm pi:prometheus/
	ssh pi "systemctl --user start logmonitor"
	ssh pi "systemctl --user status logmonitor"

.PHONY: build_arm5
build_arm5: 
	GOGCCFLAGS="-fPIC -O4 -Ofast -pipe -march=native -marm -s" \
		GOARCH=arm GOARM=5 CGO_ENABLED=1 GOOS=linux \
		CC=arm-linux-gnueabi-gcc  \
		CXX=arm-linux-gnueabi-g++ \
		CGO_LDFLAGS="-L$(RPIROOT)/lib/arm-linux-gnueabihf -lsystemd \
			-Wl,-rpath-link=$(RPIROOT)/lib/arm-linux-gnueabihf \
			-Wl,-rpath-link=$(RPIROOT)/lib/ \
			-Wl,-rpath-link=$(RPIROOT)/usr/lib/ \
			-Wl,-rpath-link=$(RPIROOT)/usr/lib/arm-linux-gnueabihf " \
		go build  $(GOTAGS) -v -o logmonitor-arm5 -ldflags $(LDFLAGS_PI)
	
.PHONY: run
run:
	#go run -v *.go -log.level debug
	CGO_ENABLED="1" go-reload `ls *.go | grep -v _test.go` -log.level debug

.PHONY: clean
clean:
	rm -f logmonitor logmonitor-arm

# vim:ft=make
