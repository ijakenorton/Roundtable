package audiomanager

import (
	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/frame"
)

// An abstraction of a lister to audio input.
// Somewhat analogous to a peer — this struct cares about receiving data from the input device,
// and may be invalidated at some point (e.g. disconnected or closed).
//
// We send data to a listener over the data channel, and use the
// context (which has a cancel function held by the peer) to check if the
// listener is invalidated.
//
// A list of listeners is held by the AudioManager, and new input data
// is fed to each listener
//
// When invalidated, the listener is removed from the listener list.
type InputListener struct {
	uuid        uuid.UUID
	dataChannel chan<- frame.PCMFrame
}
