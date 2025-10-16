# Local Peers

An example in which two peers, on the same network, establish a connection using a signalling server (also on the local network).

This acts more as a proof of concept and a test as well as an example of how to structure the clients.

### Steps

- Find a WAV file of your choice. Place it in `assets/media.wav`
- Using the [Roundtable signalling server](https://github.com/Honorable-Knights-of-the-Roundtable/signallingserver), start a signalling server on address `127.0.0.1:1066`
- In a new shell, start the answering peer on `127.0.0.1:1067`
    - Simply run `go run examples/local/answeringpeer/main.go --configFilePath examples/local/answeringpeer/config.yaml`
- In yet another new shell, start the offering peer on `127.0.0.1:1068`
    - Simply run `go run examples/local/offeringpeer/main.go --configFilePath examples/local/offeringpeer/config.yaml --audiofile assets/media.wav`
- Observe the logs printed in each shell, and note the WAV file is transmitted from offering to answering peer to be saved as the new file `connection.wav`.