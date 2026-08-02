package mixcloud

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapUser(input User) (*socialhub.User, error) {
	username, key, ok := parseUserKey(input.Key)
	if !ok || !strings.EqualFold(username, input.Username) {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud user key"))
	}
	extension, _ := json.Marshal(input)
	accountType := "basic"
	if input.IsPro {
		accountType = "pro"
	} else if input.IsPremium {
		accountType = "premium"
	}
	return &socialhub.User{
		Platform: "mixcloud", AccountID: client.accountID, ID: key,
		Username: stringPointer(input.Username), DisplayName: stringPointer(firstNonEmpty(input.Name, input.Username)),
		AvatarURL:  stringPointer(firstNonEmpty(input.Pictures.ExtraLarge, input.Pictures.Large, input.Pictures.Medium)),
		ProfileURL: stringPointer(input.URL), AccountType: stringPointer(accountType),
		Extensions: map[string]json.RawMessage{"mixcloud.user": extension},
	}, nil
}

func (client *Client) mapCloudcast(input Cloudcast) (*socialhub.Post, error) {
	username, slug, key, ok := parseCloudcastKey(input.Key)
	if !ok || input.Slug != "" && !strings.EqualFold(slug, input.Slug) {
		return nil, platformError("map_cloudcast", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud Cloudcast key"))
	}
	owner, ownerKey, ownerOK := parseUserKey(input.User.Key)
	if !ownerOK || !strings.EqualFold(owner, username) || input.User.Username != "" && !strings.EqualFold(owner, input.User.Username) {
		return nil, platformError("map_cloudcast", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud Cloudcast owner"))
	}
	extension, _ := json.Marshal(input)
	duration := time.Duration(input.AudioLength) * time.Second
	post := &socialhub.Post{
		Platform: "mixcloud", AccountID: client.accountID, ID: key, AuthorID: stringPointer(ownerKey),
		Text: stringPointer(firstNonEmpty(input.Description, input.Name)), CreatedAt: timePointer(input.CreatedTime),
		URL: stringPointer(input.URL), Extensions: map[string]json.RawMessage{"mixcloud.cloudcast": extension},
		Media: []socialhub.Media{{
			ID: key, Type: socialhub.MediaTypeAudio, Duration: durationPointer(duration), State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"mixcloud.cloudcast": extension},
		}},
	}
	if input.IsPublic != nil {
		visibility := "unlisted"
		if *input.IsPublic {
			visibility = "public"
		}
		post.Visibility = &visibility
	}
	artwork := firstNonEmpty(input.Pictures.Size1024, input.Pictures.ExtraLarge, input.Pictures.Large)
	if validHTTPURL(artwork) {
		post.Media = append(post.Media, socialhub.Media{URL: artwork, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	}
	observedAt := client.clock.Now()
	for name, value := range map[string]*int64{
		"plays": input.PlayCount, "favorites": input.FavoriteCount, "comments": input.CommentCount,
		"listeners": input.ListenerCount, "reposts": input.RepostCount,
	} {
		if value != nil {
			post.Metrics = append(post.Metrics, socialhub.Metric{
				Name: name, Value: float64(*value), AsOf: observedAt, Definition: "Mixcloud Cloudcast " + name + " total",
			})
		}
	}
	return post, nil
}

func (client *Client) mapComment(cloudcastKey string, input Comment) (socialhub.Comment, error) {
	if !validCommentKey(input.Key) {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud comment key"))
	}
	_, ownerKey, ok := parseUserKey(input.User.Key)
	if !ok {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud comment user"))
	}
	extension, _ := json.Marshal(input)
	return socialhub.Comment{
		Platform: "mixcloud", AccountID: client.accountID, ID: input.Key, PostID: cloudcastKey,
		AuthorID: stringPointer(ownerKey), Text: input.Text, CreatedAt: timePointer(input.SubmitDate),
		Extensions: map[string]json.RawMessage{"mixcloud.comment": extension},
	}, nil
}

func pageCursors(paging Paging, baseURL *url.URL) (next, previous *string, err error) {
	next, err = offsetFromPageURL(paging.Next, baseURL)
	if err != nil {
		return nil, nil, err
	}
	previous, err = offsetFromPageURL(paging.Previous, baseURL)
	if err != nil {
		return nil, nil, err
	}
	return next, previous, nil
}

func sanitizedPaging(paging Paging, baseURL *url.URL) (Paging, error) {
	for name, value := range map[string]string{"next": paging.Next, "previous": paging.Previous} {
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || baseURL == nil || !strings.EqualFold(parsed.Scheme, baseURL.Scheme) ||
			!strings.EqualFold(parsed.Host, baseURL.Host) || parsed.User != nil || parsed.Fragment != "" {
			return Paging{}, fmt.Errorf("invalid Mixcloud %s paging URL", name)
		}
		query := parsed.Query()
		query.Del("access_token")
		parsed.RawQuery = query.Encode()
		if name == "next" {
			paging.Next = parsed.String()
		} else {
			paging.Previous = parsed.String()
		}
	}
	return paging, nil
}

func offsetFromPageURL(value string, baseURL *url.URL) (*string, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || baseURL == nil || !strings.EqualFold(parsed.Scheme, baseURL.Scheme) || !strings.EqualFold(parsed.Host, baseURL.Host) || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("invalid Mixcloud paging URL")
	}
	offset := parsed.Query().Get("offset")
	if _, ok := parseOffset(offset); !ok || offset == "" {
		return nil, fmt.Errorf("Mixcloud paging URL has no valid offset")
	}
	return &offset, nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func durationPointer(value time.Duration) *time.Duration {
	if value <= 0 {
		return nil
	}
	return &value
}
