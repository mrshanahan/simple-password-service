.DEFAULT_GOAL := compile

PASSD_PORT ?= 5555

CMD_DIR = $(CURDIR)/cmd
PACKAGE_DIR = $(CURDIR)/build/package

compile:
	go build -o $(CMD_DIR)/passd $(CMD_DIR)/passd.go

build-image:
	docker build --build-arg GIT_SHA=$$(git rev-parse HEAD) -t quemot-dev/passd .

.PHONY: compile build-image
