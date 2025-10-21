.PHONY: dev_signallingserver clean build_rtaudio build_opus build_answeringpeer build_offeringpeer build_livepeer build_liveaudio all

all: build_rtaudio build_opus build_answeringpeer build_offeringpeer build_livepeer build_liveaudio

submodule_init:
	git submodule init &&\
		git submodule update &&\
		cd internal/opus &&\
		go mod tidy &&\
		go get github.com/klauspost/compress/zstd
		cd internal/rtaudio &&\
		go mod tidy &&\
		go get github.com/klauspost/compress/zstd &&\
		go mod tidy



build_rtaudio:
	go generate ./internal/rtaudio
	go build -o bin/rtaudio ./internal/rtaudio

build_opus:
	cd internal/opus && go run build.go

build_signallingserver:
	go build examples/local/signallingserver/main.go

build_answeringpeer:
	go build -tags=nolibopusfile examples/local/answeringpeer/main.go

build_offeringpeer:
	go build -tags=nolibopusfile examples/local/offeringpeer/main.go

build_livepeer:
	go build -tags=nolibopusfile examples/local/livepeer/main.go

build_liveaudio:
	go build -tags=nolibopusfile examples/local/liveaudio/main.go

signallingserver:
	go run examples/local/signallingserver/main.go

answeringpeer:
	go run -tags=nolibopusfile examples/local/answeringpeer/main.go -configFilePath examples/local/answeringpeer/config.yaml

offeringpeer:
	go run -tags=nolibopusfile examples/local/offeringpeer/main.go

livepeer:
	go run examples/local/livepeer/main.go
liveaudio:
	go run examples/local/liveaudio/main.go


rtaudio_mic_input:
	go generate ./internal/rtaudio
	go run ./examples/rtaudiodevice/main.go

rtaudio_speaker_output:
	go generate ./internal/rtaudio
	go run examples/rtaudiodevice/main.go -mode=play -file=./assets/media.wav


dev_signallingserver:
	air --build.cmd "go build -o bin/signallingserver ./cmd/signallingserver/main.go" \
		--build.full_bin "bin/signallingserver --configFilePath ./cmd/signallingserver/config.yaml" \
		--build.include_dir "cmd/signallingserver,internal"


clean:
	rm bin/*
