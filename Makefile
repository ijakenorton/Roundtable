.PHONY: dev_signallingserver clean build_rtaudio build_opus build_answeringpeer build_offeringpeer build_livepeer build_liveaudio build_all

# Run me once on clone
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

# TODO Fix needing the tag everywhere


#--------------------------------------------------------------------------------#
# Pre-build steps. 
build_all: build_rtaudio build_opus build_answeringpeer build_offeringpeer build_echoingaudiopeer build_micliveaudio

#--------------------------------------------------------------------------------#
# Only these two are necessary, the others will get built when runner anyway
build_rtaudio:
	go generate ./internal/rtaudio
	go build -o bin/rtaudio ./internal/rtaudio

build_opus:
	cd internal/opus && go run build.go
#--------------------------------------------------------------------------------#
#--------------------------------------------------------------------------------#

#--------------------------------------------------------------------------------#
# Server listening to input
build_answeringpeer:
	go build -tags=nolibopusfile examples/local/answeringpeer/main.go

# Server sending ./assets/media.wav to answeringpeer
build_offeringpeer:
	go build -tags=nolibopusfile examples/local/offeringpeer/main.go

# Server echos input to it back to the user on default output audio device
build_echoingaudiopeer:
	go build -tags=nolibopusfile examples/local/echoingaudiopeer/main.go

# Server sends default audio input data to the echoaudiopeer
build_micliveaudio:
	go build -tags=nolibopusfile examples/local/micliveaudio/main.go
#--------------------------------------------------------------------------------#

#--------------------------------------------------------------------------------#
# Server listening to input
answeringpeer:
	go run -tags=nolibopusfile examples/local/answeringpeer/main.go -configFilePath examples/local/answeringpeer/config.yaml

# Server sending ./assets/media.wav to answeringpeer
offeringpeer:
	go run -tags=nolibopusfile examples/local/offeringpeer/main.go


# Server echos input to it back to the user on default output audio device
echoingaudiopeer:
	go run -tags=nolibopusfile examples/local/echoingaudiopeer/main.go

# Server sends default audio input data to the echoaudiopeer
micliveaudio:
	go run -tags=nolibopusfile examples/local/micliveaudio/main.go

#--------------------------------------------------------------------------------#

#--------------------------------------------------------------------------------#
# Recored Audio from default mic
rtaudio_mic_input:
	go generate ./internal/rtaudio
	go run ./examples/rtaudiodevice/main.go

# Playback of audio
rtaudio_speaker_output:
	go generate ./internal/rtaudio
	go run examples/rtaudiodevice/main.go -mode=play -file=./assets/media.wav
#--------------------------------------------------------------------------------#

# Unsure if still used, I suspect using this would be helpful for all the above commands but not tried just yet
dev_signallingserver:
	air --build.cmd "go build -o bin/signallingserver ./cmd/signallingserver/main.go" \
		--build.full_bin "bin/signallingserver --configFilePath ./cmd/signallingserver/config.yaml" \
		--build.include_dir "cmd/signallingserver,internal"


#TODO  remove this; I have this here internally but should remove at somepoint
build_signallingserver:
	go build examples/local/signallingserver/main.go
signallingserver:
	go run -tags=nolibopusfile examples/local/signallingserver/main.go


clean:
	rm bin/*
