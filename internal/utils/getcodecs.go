package utils

import (
	"errors"
	"fmt"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/networking"
	"github.com/pion/webrtc/v4"
)

// Load and return a list of codecs using the given strings.
// Strings must be associated to a codec, otherwise an error is returned.
//
// See github.com/Honorable-Knights-of-the-Roundtable/Roundtable/internal/networking/codecs.go for a list of all codecs and their associated strings.
// If codecStrings is not-deduplicated, then the returned array will not be de-duplicated.
func GetUserAuthorizedCodecs(codecStrings []string) ([]webrtc.RTPCodecCapability, error) {
	if len(codecStrings) == 0 {
		return nil, errors.New("no codecs authorized")
	}

	codecs := make([]webrtc.RTPCodecCapability, len(codecStrings))
	var ok bool
	for i, s := range codecStrings {
		codecs[i], ok = networking.CodecMap[s]
		if !ok {
			return nil, fmt.Errorf("no codec with associated string %s", s)
		}
	}

	return codecs, nil
}
