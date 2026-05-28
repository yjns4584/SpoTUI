.PHONY: run build install

build:
	go build -o bin/spotui ./cmd/spotui

run: build
	./bin/spotui

install:
	go install ./cmd/spotui
