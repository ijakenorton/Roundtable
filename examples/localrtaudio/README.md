# Local Peers with Audio from Devices via RTAudio

An example in which two peers, on the same network, establish a connection using a signalling server (also on the local network). 

The `micliveaudio` peer opens the default audio input device (e.g. a microphone) with RTAudio, negotiates a connection to the `echoingaudiopeer`, and starts sending audio along that connection.

The `echoingaudiopeer` opens the default audio output device (e.g. a speaker or headphones), accepts incoming connections, and plays transmitted audio on that audio output device.

This example provides an integration test of Roundtable with RTAudio. A successful test would have the spoken audio transmitted without a degradation of quality from one peer to another. A successful test is to therefore hear ones own voice on the audio output device. Latency is also therefore tested.

### Steps

- Change directory to the `examples/localrtaudio` directory.
- Using the [Roundtable signalling server](https://github.com/Honorable-Knights-of-the-Roundtable/signallingserver), start a signalling server on address `127.0.0.1:1066`. The `Makefile` and default config file in that repo shows how to do this.
- In a new shell, start the `echoingaudiopeer` peer on `127.0.0.1:1067` (again, the given config file handles the address and port): `go run echoingaudiopeer/main.go --configFilePath echoingaudiopeer/config.yaml`
- In another new shell, start the offering peer on `127.0.0.1:1068`: `go run miclivepeer/main.go --configFilePath miclivepeer/config.yaml`
- Observe the logs printed in each shell as you speak into the default audio input device, and listen to the result on your default audio output device.