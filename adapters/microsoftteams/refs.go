package microsoftteams

import (
	"encoding/base64"
	"strings"
)

func (t Target) validate(operation string) error {
	switch t.Kind {
	case TargetChat:
		if !validOpaqueID(t.ChatID, 2048) || t.TeamID != "" || t.ChannelID != "" {
			return invalidArgument(operation, "chat target requires only a valid chat ID")
		}
	case TargetChannel:
		if !validOpaqueID(t.TeamID, 2048) || !validOpaqueID(t.ChannelID, 2048) || t.ChatID != "" {
			return invalidArgument(operation, "channel target requires valid team and channel IDs")
		}
	default:
		return invalidArgument(operation, "target kind must be chat or channel")
	}
	return nil
}

func (r MessageRef) validate(operation string, allowReply bool) error {
	if err := r.Target.validate(operation); err != nil {
		return err
	}
	if !validOpaqueID(r.RootID, 2048) {
		return invalidArgument(operation, "a valid root message ID is required")
	}
	if r.ReplyID != "" && (!allowReply || !validOpaqueID(r.ReplyID, 2048)) {
		return invalidArgument(operation, "reply message ID is not valid for this operation")
	}
	return nil
}

// ConversationRef returns an opaque, reversible common conversation ID.
func ConversationRef(target Target) (string, error) {
	if err := target.validate("conversation_ref"); err != nil {
		return "", err
	}
	if target.Kind == TargetChat {
		return "chat~" + encodeRefPart(target.ChatID), nil
	}
	return "channel~" + encodeRefPart(target.TeamID) + "~" + encodeRefPart(target.ChannelID), nil
}

// ParseConversationRef decodes a common conversation ID.
func ParseConversationRef(value string) (Target, error) {
	parts := strings.Split(value, "~")
	switch {
	case len(parts) == 2 && parts[0] == "chat":
		chatID, err := decodeRefPart(parts[1])
		if err != nil {
			return Target{}, invalidArgument("parse_conversation_ref", "invalid chat conversation reference")
		}
		target := Target{Kind: TargetChat, ChatID: chatID}
		return target, target.validate("parse_conversation_ref")
	case len(parts) == 3 && parts[0] == "channel":
		teamID, err1 := decodeRefPart(parts[1])
		channelID, err2 := decodeRefPart(parts[2])
		if err1 != nil || err2 != nil {
			return Target{}, invalidArgument("parse_conversation_ref", "invalid channel conversation reference")
		}
		target := Target{Kind: TargetChannel, TeamID: teamID, ChannelID: channelID}
		return target, target.validate("parse_conversation_ref")
	default:
		return Target{}, invalidArgument("parse_conversation_ref", "invalid Teams conversation reference")
	}
}

// EncodeMessageRef returns an opaque, reversible common message or post ID.
func EncodeMessageRef(ref MessageRef) (string, error) {
	if err := ref.validate("encode_message_ref", true); err != nil {
		return "", err
	}
	conversation, _ := ConversationRef(ref.Target)
	value := conversation + "~" + encodeRefPart(ref.RootID)
	if ref.ReplyID != "" {
		value += "~" + encodeRefPart(ref.ReplyID)
	}
	return value, nil
}

// ParseMessageRef decodes a common message or post ID.
func ParseMessageRef(value string) (MessageRef, error) {
	parts := strings.Split(value, "~")
	conversationParts := 0
	switch {
	case len(parts) >= 3 && parts[0] == "chat":
		conversationParts = 2
	case len(parts) >= 4 && parts[0] == "channel":
		conversationParts = 3
	default:
		return MessageRef{}, invalidArgument("parse_message_ref", "invalid Teams message reference")
	}
	if len(parts) != conversationParts+1 && len(parts) != conversationParts+2 {
		return MessageRef{}, invalidArgument("parse_message_ref", "invalid Teams message reference depth")
	}
	target, err := ParseConversationRef(strings.Join(parts[:conversationParts], "~"))
	if err != nil {
		return MessageRef{}, err
	}
	rootID, err := decodeRefPart(parts[conversationParts])
	if err != nil {
		return MessageRef{}, invalidArgument("parse_message_ref", "invalid root message reference")
	}
	ref := MessageRef{Target: target, RootID: rootID}
	if len(parts) == conversationParts+2 {
		ref.ReplyID, err = decodeRefPart(parts[conversationParts+1])
		if err != nil {
			return MessageRef{}, invalidArgument("parse_message_ref", "invalid reply message reference")
		}
	}
	return ref, ref.validate("parse_message_ref", true)
}

func encodeRefPart(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeRefPart(value string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return string(decoded), err
}
