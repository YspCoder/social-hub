package socialhub

import (
	"encoding/json"
	"testing"
)

func TestPostJSONPreservesStringIDsAndOptionalFields(t *testing.T) {
	t.Parallel()
	text := "hello"
	post := Post{
		Platform:  "x",
		AccountID: "primary",
		ID:        "18446744073709551615",
		Text:      &text,
	}

	encoded, err := json.Marshal(post)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Post
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != post.ID {
		t.Fatalf("ID lost precision: got %q, want %q", decoded.ID, post.ID)
	}
	if decoded.AuthorID != nil {
		t.Fatalf("missing optional author should remain nil: %#v", decoded.AuthorID)
	}
}

func TestCreatePostRequestValidate(t *testing.T) {
	t.Parallel()
	if err := (CreatePostRequest{}).Validate(); err == nil {
		t.Fatal("empty post request should fail")
	}
	empty := ""
	if err := (CreatePostRequest{Text: &empty}).Validate(); err == nil {
		t.Fatal("empty text should fail without media")
	}
	if err := (CreatePostRequest{MediaIDs: []string{"media-1"}}).Validate(); err != nil {
		t.Fatalf("media-only post should be valid: %v", err)
	}
}
