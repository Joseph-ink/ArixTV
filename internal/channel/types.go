// Package channel defines versioned snapshot values and ephemeral resolution results.
package channel

import "errors"

type StreamType string

const (
	HLS           StreamType = "hls"
	DASH          StreamType = "dash"
	Other         StreamType = "other"
	UnknownStream StreamType = "unknown"
)

func (v StreamType) Valid() bool {
	return v == HLS || v == DASH || v == Other || v == UnknownStream
}

type DRM string

const (
	Clear                DRM = "clear"
	HLSStandardEncrypted DRM = "hls_standard_encrypted"
	Widevine             DRM = "widevine"
	PlayReady            DRM = "playready"
	FairPlay             DRM = "fairplay"
	OtherDRM             DRM = "other_drm"
	UnknownDRM           DRM = "unknown"
)

func (v DRM) Valid() bool {
	switch v {
	case Clear, HLSStandardEncrypted, Widevine, PlayReady, FairPlay, OtherDRM, UnknownDRM:
		return true
	}
	return false
}

func (v DRM) Protected() bool {
	return v == Widevine || v == PlayReady || v == FairPlay || v == OtherDRM
}

type Capability string

const (
	ClearDirect       Capability = "clear_direct"
	ClearRelayOnly    Capability = "clear_relay_only"
	DRMPlayback       Capability = "drm"
	Unsupported       Capability = "unsupported"
	UnknownCapability Capability = "unknown"
)

func (v Capability) Valid() bool {
	return v == ClearDirect || v == ClearRelayOnly || v == DRMPlayback || v == Unsupported || v == UnknownCapability
}

// ValidateClassification rejects contradictory or unsafe direct/relay labels.
// Ordinary HLS AES-128 is not a DRM license system. Providers must independently
// establish normal-client usability before assigning ClearDirect.
func ValidateClassification(t StreamType, drm DRM, capability Capability) error {
	if !t.Valid() || !drm.Valid() || !capability.Valid() {
		return errors.New("invalid stream classification")
	}
	if drm == HLSStandardEncrypted && t != HLS {
		return errors.New("standard HLS encryption requires HLS")
	}
	if capability == ClearDirect || capability == ClearRelayOnly {
		if (drm != Clear && drm != HLSStandardEncrypted) || t == UnknownStream {
			return errors.New("clear playback requires a known clear classification")
		}
	}
	if capability == DRMPlayback && !drm.Protected() {
		return errors.New("DRM capability requires protected-content classification")
	}
	if drm.Protected() && capability != DRMPlayback && capability != Unsupported {
		return errors.New("protected content must be DRM or unsupported")
	}
	return nil
}
