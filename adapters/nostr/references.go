package nostr

import (
	nostrgo "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip10"
)

type eventReference struct {
	ID     nostrgo.ID
	Relay  string
	Author nostrgo.PubKey
}

func threadRootReference(tags nostrgo.Tags) (eventReference, bool) {
	for _, tag := range tags {
		if reference, ok := markedReference(tag, "root"); ok {
			return reference, true
		}
	}
	if pointer := eventPointer(nip10.GetThreadRoot(tags)); pointer != nil {
		return pointerReference(*pointer), true
	}
	return eventReference{}, false
}

func immediateParentReference(tags nostrgo.Tags) (eventReference, bool) {
	for _, tag := range tags {
		if reference, ok := markedReference(tag, "reply"); ok {
			return reference, true
		}
	}
	if root, ok := threadRootReference(tags); ok {
		return root, true
	}
	if pointer := eventPointer(nip10.GetImmediateParent(tags)); pointer != nil {
		return pointerReference(*pointer), true
	}
	return eventReference{}, false
}

func markedReference(tag nostrgo.Tag, marker string) (eventReference, bool) {
	if len(tag) < 4 || tag[0] != "e" || tag[3] != marker {
		return eventReference{}, false
	}
	identifier, err := nostrgo.IDFromHex(tag[1])
	if err != nil {
		return eventReference{}, false
	}
	reference := eventReference{ID: identifier, Relay: tag[2]}
	if len(tag) >= 5 {
		reference.Author, _ = nostrgo.PubKeyFromHex(tag[4])
	}
	return reference, true
}

func pointerReference(pointer nostrgo.EventPointer) eventReference {
	reference := eventReference{ID: pointer.ID, Author: pointer.Author}
	if len(pointer.Relays) > 0 {
		reference.Relay = pointer.Relays[0]
	}
	return reference
}

func referenceTag(name string, reference eventReference, marker string) nostrgo.Tag {
	author := ""
	if reference.Author != nostrgo.ZeroPK {
		author = reference.Author.Hex()
	}
	return relayTag(name, reference.ID.Hex(), reference.Relay, marker, author)
}
