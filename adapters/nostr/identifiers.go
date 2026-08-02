package nostr

import (
	"strings"

	nostrgo "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

func parseSecretKey(value string) (nostrgo.SecretKey, error) {
	value = strings.TrimSpace(value)
	if len(value) == 64 {
		return nostrgo.SecretKeyFromHex(value)
	}
	prefix, decoded, err := nip19.Decode(value)
	if err != nil || prefix != "nsec" {
		return nostrgo.SecretKey{}, errInvalidIdentifier
	}
	secret, ok := decoded.(nostrgo.SecretKey)
	if !ok || secret.Public() == nostrgo.ZeroPK {
		return nostrgo.SecretKey{}, errInvalidIdentifier
	}
	return secret, nil
}

func parsePublicKey(value string) (nostrgo.PubKey, error) {
	value = strings.TrimSpace(value)
	if len(value) == 64 {
		return nostrgo.PubKeyFromHex(value)
	}
	prefix, decoded, err := nip19.Decode(value)
	if err != nil {
		return nostrgo.PubKey{}, errInvalidIdentifier
	}
	switch prefix {
	case "npub":
		publicKey, ok := decoded.(nostrgo.PubKey)
		if ok && publicKey != nostrgo.ZeroPK {
			return publicKey, nil
		}
	case "nprofile":
		profile, ok := decoded.(nostrgo.ProfilePointer)
		if ok && profile.PublicKey != nostrgo.ZeroPK {
			return profile.PublicKey, nil
		}
	}
	return nostrgo.PubKey{}, errInvalidIdentifier
}

func parseEventID(value string) (nostrgo.ID, error) {
	value = strings.TrimSpace(value)
	if len(value) == 64 {
		return nostrgo.IDFromHex(value)
	}
	prefix, decoded, err := nip19.Decode(value)
	if err != nil {
		return nostrgo.ID{}, errInvalidIdentifier
	}
	if prefix == "note" || prefix == "nevent" {
		pointer, ok := decoded.(nostrgo.EventPointer)
		if ok && pointer.ID != nostrgo.ZeroID {
			return pointer.ID, nil
		}
	}
	return nostrgo.ID{}, errInvalidIdentifier
}
