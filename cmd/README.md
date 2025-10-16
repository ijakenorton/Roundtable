# Roundtable Client

The client of the Roundtable app. This is what is run by each peer, manages peer to peer connections (once established with the help of the public signalling server), and reads/writes audio data to/from hardware devices.

### Config

| Key | Datatype    | Default Value | Description   |
| --- | ---         | ---           | ---           |
| loglevel | String Enum ("none", "error", "warn", "info", "debug") | "info" | The level at which logs are recorded. None disables logging. |
| logfile | String | nil | The filepath to write logs to. If left unset or empty, logs are sent to `stdout`. The file is truncated before logging begins. If the file cannot be opened for writing, the program panics. |
| ICEServers | List of Strings | nil | Required. At least one ICE server must be specified, otherwise the program will panic during initialization. Specify the protocol alongside the server, e.g. `"stun:stun.l.google.com:19302"`, `"turn:127.0.0.1:1234"`.<br />See below for a description of STUN vs TURN. |
| timeout | int | 30 | Defines the time (in seconds) to wait for a request before timing out. |
| Codecs | list of Strings | ["CodecOpus48000Stereo", "CodecOpus48000Mono"] | Define the audio codecs to be used when negotiating a connection. The first codec specified is the preferred (but not guaranteed) codec for connections.<br />Be warned, at least one codec must be common to both peers for a connection to be formed! Furthermore, in general, the higher the sample rate, the higher than bandwidth (same for stereo vs mono).<br />The valid codecs are: "CodecOpus48000Stereo", "CodecOpus48000Mono". |
| signallingserver | String | nil | Required. Defines the publicly available IP (or resolvable domain name) and port of the signalling server, defined under `github.com/Honorable-Knights-of-the-Roundtable/roundtable/cmd/signallingserver`.<br />This server forwards SDP offers and answers between roundtable clients, which allows for the connection of users together even behind NAT.<br />e.g. `http://127.0.0.1:1066`.|
| localport | int | 1066 | Defines the local port number to bind to for listening to incoming peer connections from the signalling server. |