# Roundtable

### A Peer-to-Peer VoIP Application

---

Roundtable is under active development. In its current state, the application is not ready for deployment. The structure of the program may change drastically during this phase of development.

# Plan

The aim of Roundtable is to allow for Peer-to-Peer (P2P), and hence low-latency, communication of voice. This requires several key modules to be implemented:

### Getting Voice Data
- Obtain signal/data from the microphone/hardware device
    - This should have cross-platform support!
    - Check for libraries to make this aspect easier
    - Testing: it may be possible to spoof the hardware device for testing purposes... for later stages of Ingress, it should be possible to have a static audio file that is loaded for testing.

- Transform data into a serializable format to send to peers
    - This may depend on the P2P networking library used, and what formats it supports
    - Testing: this aspect should be easy enough to test with static files or perhaps fuzzing

- Send data to peers
    - Using what ever P2P library is decided upon, send the serialized data to all peers
    - Realistically this is going to be done in a streaming-like manner to avoid waiting
    - Sending should be handled by a distinct thread per peer to minimize latency
    - The P2P library may require buffering or other logic, which could mean additional work in packaging the data before sending
    - Testing: it may be possible to spoof the networking interface here, to avoid actually setting up a connection/peer

### Playing Peer Data
- Read data from each peer
    - Just like sending data to each peer, this should be threaded to minimize latency, then any results should be fed into one parent thread for consolidation
    - Testing: spoofing the network may be possible again... consider what happens when there are many peers?

- Deserialize peer data into a useful format for playback
    - The source format is going to be entirely determined by the P2P library, but the final format could be any number of things...
    - Depending on the playback method, the format may be more restrictive
    - If the network delivers data in a stream, is it possible to play the most recent fragment and still achieve a useful user experience?
    - Testing: static files and fuzzing should be sufficient for testing transformations

- Combine all peer fragments into one stream ready for playback, apply any filters
    - If two peers send data at once, they should be played simultaneously. Combining two distinct data streams should be investigated
    - It may be possible to apply filters (e.g. noise reduction) to the data stream before playing it to the user
    - Testing: combining signals may be easy to test with static files, same for filters

- Send final data stream to audio output hardware
    - Just like input, this should be cross-platform
    - Testing: TBD

### Networking
P2P Networking is non-trivial... consider WebRTC as a foundation. Alternatively, something based on ConnectRPC may be applicable.

Testing throughout this section is going to be extremely difficult!

- Determine public facing IP
    - P2P requires punching through NAT to allow connections, since almost all users will not have a static public IP
    - STUN servers seem to allow for this, some are publicly available, and it doesn't seem to be a privacy issue to use these

- Set up connections to peers
    - Once a user has started the application, how can they connect to another user? 
    - What about to an existing group of users?
    - What about a user in a group adding a new user?
    - Consider security... what if a bad actor tries to connect? How should a connection be verified / accepted?

### Other Factors

- Config
    - A nice configuration method would be ideal, such as selecting STUN servers, setting any security for incoming connections, and so on

- Security
    - Require a password or token for incoming connections?
    - Lock a group to prevent any new connections?
    - Rate limit an IP address to avoid a bad-actor hammering a user?

- Various user-experience methods
    - Mute, deafen, etc...
    - Per-user equalizer, i.e. set a volume level per user
    - Notification sounds for a new connection, a lost connection, etc.

- User interface
    - Interactions beyond simple audio-in, audio-out appear to be complex enough to warrant a user interface
    - Something terminal based, which could be bound to underlying data models with ease, could be useful
    - A cross-platform GUI could be more difficult to implement correctly


### ICE, STUN, and TURN

ICE (Interactive Connectivity Establishment) is a framework that coordinates how peers discover and test possible connection paths. The two most common methods used by ICE are STUN and TURN, described below. ICE operates to find network candidates (valid connections between peers) and select the best such candidate, even through complex public network infrastructure like NAT.

STUN (Session Traversal Utilities for NAT) simply responds to an incoming request with the corresponding public IP address and port number, i.e. STUN tells the requesting machine how it appears to the public internet. Using STUN servers establishes true peer-to-peer connections, as peers can connect directly to one another without relying on a middleman. However, if both peers have highly restrictive network infrastructure (e.g. firewalls that drop UDP traffic) then direct connections may not be possible. Typically, STUN is faster than TURN, and does not rely on a publicly available relaying server. Many public STUN servers are available and can be used for free.

TURN (Traversal Using Relays around NAT) acts as a middleman between peers and forwarding packets from one peer to another. TURN, therefore, is more robust to network infrastructure than STUN, but requires more infrastructure (a publicly available TURN server), more bandwidth (all packets are forwarded back and forth to the server) and may be higher latency than a direct connection. Consider using STUN first, with a TURN server as a backup. Due to the bandwidth demands of TURN, publicly available servers are less common. Setting up a TURN server is not difficult, and opensource implementations exist, e.g. [coturn](https://github.com/coturn/coturn).

# Dependencies

### Development Dependencies

- `opus` is used as an audio encoder. To build this project, the opus development libraries are required. This may be achieved directly (via the [opus-codec](https://opus-codec.org/downloads/) site) or through your package manager. For example, on dnf, run `dnf install opus-devel opusfile-devel`. On apt, run `apt-get install opus-tools libopus0 libopus-dev`.

##### Optional

- `Make` is used to help build aspects of this project.
- [`air`](https://github.com/air-verse/air), a hot-reload tool, is used in some development commands within the Makefile, but this is optional.
