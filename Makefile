.PHONY: dev_signallingserver clean

dev_signallingserver:
	air --build.cmd "go build -o bin/signallingserver ./cmd/signallingserver/main.go" \
		--build.full_bin "bin/signallingserver --configFilePath ./cmd/signallingserver/config.yaml" \
		--build.include_dir "cmd/signallingserver,internal"

build_rtaudio:
	go generate ./internal/rtaudio
	go build -o bin/rtaudio ./internal/rtaudio

dev_rtaudio:
	go generate ./internal/rtaudio
	go run ./examples/rtaudiodevice/main.go

clean:
	rm bin/*
