package nostr

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	nostrgo "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"

	"social-hub/pkg/socialhub"
)

type profileMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Picture     string `json:"picture"`
	Website     string `json:"website"`
	About       string `json:"about"`
	NIP05       string `json:"nip05"`
	Banner      string `json:"banner"`
	LUD06       string `json:"lud06"`
	LUD16       string `json:"lud16"`
}

func (client *Client) mapUser(event nostrgo.Event, report queryReport) (*socialhub.User, error) {
	var metadata profileMetadata
	if err := json.Unmarshal([]byte(event.Content), &metadata); err != nil {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	identifier := event.PubKey.Hex()
	profileURI := "nostr:" + nip19.EncodeNpub(event.PubKey)
	user := &socialhub.User{
		Platform: "nostr", AccountID: client.accountID, ID: identifier,
		Username: optionalString(metadata.Name), DisplayName: optionalString(metadata.DisplayName),
		AvatarURL: optionalHTTPURL(metadata.Picture), ProfileURL: &profileURI,
		AccountType: stringPointer("decentralized_public_key"), Extensions: eventExtensions(event, report),
	}
	putExtension(user.Extensions, "nostr.npub", nip19.EncodeNpub(event.PubKey))
	putExtension(user.Extensions, "nostr.metadata", metadata)
	return user, nil
}

func (client *Client) mapPost(event nostrgo.Event, report queryReport) socialhub.Post {
	authorID, text := event.PubKey.Hex(), event.Content
	visibility := "public"
	postURI := "nostr:" + nip19.EncodeNevent(event.ID, report.Sources[event.ID.Hex()], event.PubKey)
	post := socialhub.Post{
		Platform: "nostr", AccountID: client.accountID, ID: event.ID.Hex(), AuthorID: &authorID,
		Text: &text, Media: mapMedia(event), Relations: mapRelations(event), CreatedAt: eventTime(event.CreatedAt),
		URL: &postURI, Visibility: &visibility, Extensions: eventExtensions(event, report),
	}
	return post
}

func (client *Client) mapComment(event nostrgo.Event, postID string, report queryReport) socialhub.Comment {
	authorID := event.PubKey.Hex()
	comment := socialhub.Comment{
		Platform: "nostr", AccountID: client.accountID, ID: event.ID.Hex(), PostID: postID,
		AuthorID: &authorID, Text: event.Content, CreatedAt: eventTime(event.CreatedAt),
		Extensions: eventExtensions(event, report),
	}
	if parent, found := immediateParentReference(event.Tags); found && parent.ID.Hex() != postID {
		parentID := parent.ID.Hex()
		comment.ParentID = &parentID
	}
	return comment
}

func mapRelations(event nostrgo.Event) []socialhub.PostRelation {
	relations := make([]socialhub.PostRelation, 0, 2)
	if event.Kind == nostrgo.KindTextNote {
		if parent, found := immediateParentReference(event.Tags); found {
			relations = appendRelation(relations, socialhub.RelationReply, parent.ID.Hex())
		}
	}
	for tag := range event.Tags.FindAll("q") {
		if len(tag) < 2 {
			continue
		}
		if identifier, err := nostrgo.IDFromHex(tag[1]); err == nil {
			relations = appendRelation(relations, socialhub.RelationQuote, identifier.Hex())
		}
	}
	if event.Kind == nostrgo.KindRepost {
		for tag := range event.Tags.FindAll("e") {
			if len(tag) < 2 {
				continue
			}
			if identifier, err := nostrgo.IDFromHex(tag[1]); err == nil {
				relations = appendRelation(relations, socialhub.RelationRepost, identifier.Hex())
				break
			}
		}
	}
	return relations
}

func appendRelation(relations []socialhub.PostRelation, relationType socialhub.RelationType, identifier string) []socialhub.PostRelation {
	for _, relation := range relations {
		if relation.Type == relationType && relation.PostID == identifier {
			return relations
		}
	}
	return append(relations, socialhub.PostRelation{Type: relationType, PostID: identifier})
}

func eventPointer(pointer nostrgo.Pointer) *nostrgo.EventPointer {
	switch value := pointer.(type) {
	case nostrgo.EventPointer:
		return &value
	case *nostrgo.EventPointer:
		return value
	default:
		return nil
	}
}

func eventExtensions(event nostrgo.Event, report queryReport) map[string]json.RawMessage {
	extensions := make(map[string]json.RawMessage, 3)
	putExtension(extensions, "nostr.event", event)
	if sources := report.Sources[event.ID.Hex()]; len(sources) > 0 {
		putExtension(extensions, "nostr.relays", sources)
	}
	if len(report.Failed) > 0 {
		putExtension(extensions, "nostr.partial_failures", report.Failed)
	}
	return extensions
}

func mapMedia(event nostrgo.Event) []socialhub.Media {
	media := make([]socialhub.Media, 0)
	for tag := range event.Tags.FindAll("imeta") {
		fields, fallbacks := parseIMeta(tag)
		mediaURL := fields["url"]
		if !validRemoteURL(mediaURL) || !strings.Contains(event.Content, mediaURL) {
			continue
		}
		mime := fields["m"]
		item := socialhub.Media{URL: mediaURL, MIME: mime, Type: mediaType(mime), State: socialhub.MediaStateReady}
		item.ID = fields["x"]
		if size, err := strconv.ParseInt(fields["size"], 10, 64); err == nil && size >= 0 {
			item.Size = &size
		}
		if width, height, ok := parseDimensions(fields["dim"]); ok {
			item.Width, item.Height = &width, &height
		}
		item.Extensions = make(map[string]json.RawMessage)
		for _, key := range []string{"alt", "blurhash", "thumb", "image", "summary", "x"} {
			if value := fields[key]; value != "" {
				putExtension(item.Extensions, "nostr."+key, value)
			}
		}
		if len(fallbacks) > 0 {
			putExtension(item.Extensions, "nostr.fallback", fallbacks)
		}
		if len(item.Extensions) == 0 {
			item.Extensions = nil
		}
		media = append(media, item)
	}
	return media
}

func parseIMeta(tag nostrgo.Tag) (map[string]string, []string) {
	fields := make(map[string]string)
	var fallbacks []string
	for _, value := range tag[1:] {
		key, field, found := strings.Cut(value, " ")
		if found && key != "" && field != "" {
			if key == "fallback" {
				fallbacks = append(fallbacks, field)
			} else {
				fields[key] = field
			}
		}
	}
	return fields, fallbacks
}

func parseDimensions(value string) (int, int, bool) {
	widthValue, heightValue, found := strings.Cut(strings.ToLower(value), "x")
	if !found {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(widthValue)
	height, heightErr := strconv.Atoi(heightValue)
	return width, height, widthErr == nil && heightErr == nil && width > 0 && height > 0
}

func mediaType(mime string) socialhub.MediaType {
	switch {
	case strings.HasPrefix(strings.ToLower(mime), "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(strings.ToLower(mime), "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(strings.ToLower(mime), "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeDocument
	}
}

func validRemoteURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User == nil
}

func optionalHTTPURL(value string) *string {
	if !validRemoteURL(value) {
		return nil
	}
	return &value
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPointer(value string) *string { return &value }

func putExtension(target map[string]json.RawMessage, key string, value any) {
	encoded, err := json.Marshal(value)
	if err == nil {
		target[key] = encoded
	}
}
