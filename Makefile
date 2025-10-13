.PHONY: dev_signallingserver build_audio dev_audio

dev_signallingserver:
	air --build.cmd "go build -o bin/signallingserver ./cmd/signallingserver/main.go" \
		--build.full_bin "bin/signallingserver --configFilePath ./cmd/signallingserver/config.yaml" \
		--build.include_dir "cmd/signallingserver,internal"

build_audio:
	go generate ./internal/audio
	go build -o bin/audio ./internal/audio

dev_audio:
	go generate ./internal/audio
	go run ./cmd/audio/
