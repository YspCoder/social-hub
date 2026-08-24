package viber

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTypedMessageValidation(t *testing.T) {
	valid := []MessageObject{
		TextMessage{Text: "hello"},
		PictureMessage{MediaURL: "https://cdn.example/image.png"},
		VideoMessage{MediaURL: "https://cdn.example/video.mp4", Size: 10, Duration: time.Second},
		FileMessage{MediaURL: "https://cdn.example/file.pdf", Size: 10, Filename: "file.pdf"},
		ContactMessage{Name: "Ada", PhoneNumber: "+441234567"},
		LocationMessage{Latitude: 51.5, Longitude: -0.1},
		URLMessage{URL: "https://example.com"},
		StickerMessage{StickerID: 46105},
	}
	for index, message := range valid {
		body, err := message.viberMessage()
		if err != nil || body["type"] == "" {
			t.Fatalf("valid message %d body=%#v error=%v", index, body, err)
		}
	}
	invalid := []MessageObject{
		TextMessage{},
		TextMessage{Text: strings.Repeat("x", 7001)},
		PictureMessage{MediaURL: "https://cdn.example/image.webp"},
		PictureMessage{MediaURL: "https://cdn.example/image.png", ThumbnailURL: "ftp://cdn.example/t.jpg"},
		VideoMessage{MediaURL: "https://cdn.example/video.mov", Size: 1},
		VideoMessage{MediaURL: "https://cdn.example/video.mp4", Size: maxVideoBytes + 1},
		VideoMessage{MediaURL: "https://cdn.example/video.mp4", Size: 1, Duration: 181 * time.Second},
		VideoMessage{MediaURL: "https://cdn.example/video.mp4", Size: 1, Duration: time.Millisecond},
		FileMessage{MediaURL: "https://cdn.example/file", Size: 1, Filename: "folder/file.pdf"},
		FileMessage{MediaURL: "https://cdn.example/file", Size: maxFileBytes + 1, Filename: "file.pdf"},
		ContactMessage{Name: "", PhoneNumber: "1"},
		ContactMessage{Name: "Ada", PhoneNumber: strings.Repeat("1", 19)},
		LocationMessage{Latitude: math.NaN()},
		LocationMessage{Latitude: 0, Longitude: 181},
		URLMessage{URL: "javascript:alert(1)"},
		StickerMessage{},
	}
	for index, message := range invalid {
		if _, err := message.viberMessage(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid message %d error=%v", index, err)
		}
	}
}

func TestRecipientWebhookAndErrorValidation(t *testing.T) {
	if _, err := validateRecipients(nil, 10, "test"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty recipients=%v", err)
	}
	if _, err := validateRecipients([]string{"a", "a"}, 10, "test"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("duplicate recipients=%v", err)
	}
	if _, err := validateRecipients([]string{"a b"}, 10, "test"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid recipient=%v", err)
	}
	if _, err := validateEventTypes([]WebhookEventType{WebhookMessage, WebhookMessage}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("duplicate events=%v", err)
	}
	if _, err := validateEventTypes([]WebhookEventType{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("unknown event=%v", err)
	}
	if validWebhookURL("http://example.com") || validWebhookURL("https://user:pass@example.com") || !validWebhookURL("https://example.com/hook") {
		t.Fatal("webhook URL validation mismatch")
	}
	if code := errorCode(mapStatus("test", http.StatusOK, 6, "not subscribed")); code != socialhub.CodePermissionDenied {
		t.Fatalf("status 6 code=%s", code)
	}
	if code := errorCode(mapStatus("test", http.StatusOK, 10, "webhook not set")); code != socialhub.CodeConflict {
		t.Fatalf("status 10 code=%s", code)
	}
	if code := errorCode(mapStatus("test", http.StatusOK, 99, "unknown")); code != socialhub.CodePlatformError {
		t.Fatalf("status 99 code=%s", code)
	}
	if delay := parseRetryAfter("1.5"); delay != 1500*time.Millisecond {
		t.Fatalf("retry after=%s", delay)
	}
	if delay := parseRetryAfter("invalid"); delay != 0 {
		t.Fatalf("invalid retry after=%s", delay)
	}
	if got := boundedMessage(strings.Repeat("界", 513), 512); len([]rune(got)) != 512 {
		t.Fatalf("bounded runes=%d", len([]rune(got)))
	}
}
