package signalling

import "github.com/google/uuid"

type PeerIdentifier struct {
	Uuid     uuid.UUID
	PublicIP string
}
