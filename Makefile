.PHONY: dev_signallingserver clean

answeringpeer:
	go run -tags=nolibopusfile examples/local/answeringpeer/main.go -configFilePath examples/local/answeringpeer/config.yaml

build_opus:
	cd internal/opus && go build build.go


dev_signallingserver:
	air --build.cmd "go build -o bin/signallingserver ./cmd/signallingserver/main.go" \
		--build.full_bin "bin/signallingserver --configFilePath ./cmd/signallingserver/config.yaml" \
		--build.include_dir "cmd/signallingserver,internal"

signallingserver:
	go run examples/local/signallingserver/main.go

offeringpeer:
	go run examples/local/offeringpeer/main.go


livepeer:
	go run examples/local/livepeer/main.go
liveaudio:
	go run examples/local/liveaudio/main.go

build_rtaudio:
	go generate ./internal/rtaudio
	go build -o bin/rtaudio ./internal/rtaudio

rtaudio_mic_input:
	go generate ./internal/rtaudio
	go run ./examples/rtaudiodevice/main.go

rtaudio_speaker_output:
	go generate ./internal/rtaudio
	go run examples/rtaudiodevice/main.go -mode=play -file=./assets/media.wav

clean:
	rm bin/*
