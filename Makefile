#
# Makefile
#
#
#
VERSION=dev
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

RPIROOT=/home/k/mnt/pi/

# enable/disable sdjournal
GOTAGS=-tags 'sdjournal'

#	-rpath=/home/k/mnt/pi/lib/arm-linux-gnueabihf \
#	-rpath-link=/home/k/mnt/pi/lib/arm-linux-gnueabihf \
#	-sysroot=/home/k/mnt/pi/ \
#	-L/home/k/mnt/pi/lib/arm-linux-gnueabihf/ \

build: 
	CGO_ENABLED=1 go build $(GOTAGS) -v -o logmonitor -ldflags $(LDFLAGS)

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

run:
	#go run -v *.go -log.level debug
	CGO_ENABLED="1" go-reload `ls *.go | grep -v _test.go` -log.level debug

clean:
	rm -f logmonitor logmonitor-arm

# vim:ft=make
