# Roundtable Signalling Server

A very basic signalling server to be hosted publicly. This server forwards SDP offers from one client to another, and responds with the corresponding answer, allowing for peer to peer connections to be made.

Ideally this server is hosted on a publicly available IP address that is behind some kind of DDOS protection (e.g. Cloudflare) to avoid malicious agents using this server in an amplification attack on unsuspecting remote clients.

Beware, if hiding this server behind a port-forwarder/reverse proxy, to give your clients the *publicly available* address for the signalling server, including the external port number!!

# Config

| Key | Datatype    | Default Value | Description   |
| --- | ---         | ---           | ---           |
| loglevel | String Enum ("none", "error", "warn", "info", "debug") | "info" | The level at which logs are recorded. None disables logging. |
| logfile | String | nil | The filepath to write logs to. If left unset or empty, logs are sent to `stdout`. The file is truncated before logging begins. If the file cannot be opened for writing, the program panics. |
| localaddress | string | localhost:1066 | Defines the local address (including port number) to bind to for listening to incoming peer connections from the signalling server. |