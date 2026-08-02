package nostr

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"

	nostrgo "fiatjaf.com/nostr"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func signNIP01Event(event *nostrgo.Event, secret nostrgo.SecretKey) error {
	if event.Tags == nil {
		event.Tags = make(nostrgo.Tags, 0)
	}
	privateKey, publicKey := btcec.PrivKeyFromBytes(secret[:])
	event.PubKey = nostrgo.PubKey(publicKey.SerializeCompressed()[1:])
	event.ID = canonicalEventID(*event)
	signature, err := schnorr.Sign(privateKey, event.ID[:], schnorr.FastSign())
	if err != nil {
		return err
	}
	copy(event.Sig[:], signature.Serialize())
	return nil
}

func validNIP01Event(event nostrgo.Event) bool {
	if event.ID != canonicalEventID(event) {
		return false
	}
	publicKey, err := schnorr.ParsePubKey(event.PubKey[:])
	if err != nil {
		return false
	}
	signature, err := schnorr.ParseSignature(event.Sig[:])
	return err == nil && signature.Verify(event.ID[:], publicKey)
}

func canonicalEventID(event nostrgo.Event) nostrgo.ID {
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode([]any{0, event.PubKey.Hex(), event.CreatedAt, event.Kind, event.Tags, event.Content})
	serialized := bytes.TrimSuffix(payload.Bytes(), []byte{'\n'})
	digest := sha256.Sum256(serialized)
	return nostrgo.ID(digest)
}
