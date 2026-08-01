package bilibili

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestSignerAddsV2HeadersAndBodyDigest(t *testing.T) {
	body := []byte(`{"name":"video.mp4","utype":"1"}`)
	request, err := http.NewRequest(http.MethodPost, "https://member.example/init", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	signer := &requestSigner{clientID: "client-id", appSecret: "app-secret", clock: fixedClock{now: time.Unix(1785542400, 0)}}
	if err := signer.Authenticate(request, socialhub.Token{AccessToken: "access-token"}); err != nil {
		t.Fatal(err)
	}
	restored, _ := io.ReadAll(request.Body)
	if !bytes.Equal(restored, body) {
		t.Fatalf("restored body=%q", restored)
	}
	sum := md5.Sum(body)
	if request.Header.Get("X-Bili-Content-Md5") != hex.EncodeToString(sum[:]) || request.Header.Get("X-Bili-Timestamp") != "1785542400" || request.Header.Get("Access-Token") != "access-token" {
		t.Fatalf("headers=%v", request.Header)
	}
	headers := signedHeaders(request)
	if request.Header.Get("Authorization") != signatureFor(headers, "app-secret") {
		t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
	}
	if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content headers=%v", request.Header)
	}
}

func signedHeaders(request *http.Request) map[string]string {
	return map[string]string{
		"x-bili-accesskeyid":       request.Header.Get("X-Bili-Accesskeyid"),
		"x-bili-content-md5":       request.Header.Get("X-Bili-Content-Md5"),
		"x-bili-signature-method":  request.Header.Get("X-Bili-Signature-Method"),
		"x-bili-signature-nonce":   request.Header.Get("X-Bili-Signature-Nonce"),
		"x-bili-signature-version": request.Header.Get("X-Bili-Signature-Version"),
		"x-bili-timestamp":         request.Header.Get("X-Bili-Timestamp"),
	}
}
