# Local Peers

An example in which two peers, on the same network, establish a connection using a signalling server (also on the local network). The offering peer opens a WAV file (to be supplied), reads the audio data into memory, makes a connection via the signalling server to the answering peer, and plays the file. The answering peer listens for new connections, accepts the incoming audio data, and sends that data to another file on disk. This allows for testing of encoding, sending, receiving, and decoding audio data in one integrated pipeline.

This acts more as a proof of concept and a test as well as an example of how to structure the clients.

### Steps

- Change directory to the `examples/localfileaudio` directory.
- Locate a WAV file. Files may have any sampling rate, but should be mono or stereo (one or two channels). Note the path to the audio file (e.g. `assets/media.wav`)
- Using the [Roundtable signalling server](https://github.com/Honorable-Knights-of-the-Roundtable/signallingserver), start a signalling server on address `127.0.0.1:1066`. The `Makefile` and default config file in that repo shows how to do this.
- In a new shell, start the answering peer on `127.0.0.1:1067` (again, the given config file handles the address and port): `go run answeringpeer/main.go --configFilePath answeringpeer/config.yaml`
- In another new shell, start the offering peer on `127.0.0.1:1068`: `go run offeringpeer/main.go --configFilePath offeringpeer/config.yaml --audiofile assets/media.wav`
- Observe the logs printed in each shell, and note the WAV file is transmitted from offering to answering peer to be saved as the new file `connection.wav`.