GO_LDFLAGS=-ldflags " -w"
VERSION=1.0.1
BIN_PATH=./bin
MAC_BIN_PATH=./bin/Mac/${VERSION}
LUX_BIN_PATH=./bin/Lux/${VERSION}
WIN_BIN_PATH=./bin/WIN/${VERSION}

default: help

all: build-mac build-lux  build-kerri-win push-lux

clean: 
	@rm -rf ${BIN_PATH}/*

build-kerri-lux: 
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${GO_LDFLAGS} -o ${LUX_BIN_PATH}/kerrigan kerrigan.go
build-webcli-lux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${GO_LDFLAGS} -o ${LUX_BIN_PATH}/kerrictl kerrigan-cli.go
build-kerri-mac: 
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GO_LDFLAGS} -o ${MAC_BIN_PATH}/kerrigan kerrigan.go
build-webcli-mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GO_LDFLAGS} -o ${MAC_BIN_PATH}/kerrictl kerrigan-cli.go
build-kerri-win: 
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ${GO_LDFLAGS} -o ${WIN_BIN_PATH}/kerrigan.exe kerrigan.go
build-webcli-win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ${GO_LDFLAGS} -o ${WIN_BIN_PATH}/kerrictl.exe kerrigan-cli.go
build-mac: build-kerri-mac build-webcli-mac	
build-lux: build-kerri-lux build-webcli-lux
build-win: build-kerri-win build-webcli-win
push-lux:
	@scp ${LUX_BIN_PATH}/kerrigan ${LUX_BIN_PATH}/kerrictl root@150.109.11.142:/root

help:
	@echo ""
	@echo "Build Usage:"
	@echo "    ..............................................." 
	@echo "\033[35m    make [build-mac / build-win / build-lux / all] \033[0m"
	@echo "    ..............................................." 
	@echo "\033[32m    build-mac \033[0m" "\033[36m to build binary program under Mac OS platform. \033[0m"
	@echo "\033[32m    build-lux \033[0m" "\033[36m to build binary program under Linux platform.   \033[0m"
	@echo "\033[32m    build-win \033[0m" "\033[36m to build binary program under Windows platform. \033[0m"
	@echo "\033[32m    all       \033[0m" "\033[36m to build all programs binary programs.  \033[0m"
	@echo ""