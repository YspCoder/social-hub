package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestBotWorkflowGetMeMediaAndWebhookRegistration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/bot123456:bot-token/getMe":
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"Hub Bot","username":"hub_bot"}}`))
		case "/bot123456:bot-token/sendPhoto":
			if !parseTelegramForm(t, writer, request) {
				return
			}
			if request.FormValue("chat_id") != "-1001" || request.FormValue("photo") != "photo-file-id" || request.FormValue("caption") != "caption" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":50,"date":1785542400,"chat":{"id":-1001,"type":"group"},"photo":[{"file_id":"small","file_unique_id":"u1","width":100,"height":100,"file_size":1000},{"file_id":"large","file_unique_id":"u2","width":1000,"height":800,"file_size":5000}],"caption":"caption"}}`))
		case "/bot123456:bot-token/sendDocument":
			if !parseTelegramForm(t, writer, request) {
				return
			}
			files := request.MultipartForm.File["document"]
			if len(files) != 1 || files[0].Filename != "report.pdf" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, err := files[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(file)
			_ = file.Close()
			if string(body) != "pdf-data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":51,"date":1785542400,"chat":{"id":-1001,"type":"group"},"document":{"file_id":"doc-id","file_unique_id":"doc-u","file_name":"report.pdf","mime_type":"application/pdf","file_size":8}}}`))
		case "/bot123456:bot-token/setWebhook":
			if !parseTelegramForm(t, writer, request) {
				return
			}
			if request.FormValue("url") != "https://app.example/telegram" || request.FormValue("secret_token") != "webhook_secret-1" || request.FormValue("max_connections") != "25" || request.FormValue("allowed_updates") != `["message","callback_query"]` {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":true}`))
		case "/bot123456:bot-token/deleteWebhook":
			if !parseTelegramForm(t, writer, request) {
				return
			}
			if request.FormValue("drop_pending_updates") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":true}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "-1001", true)
	user, err := client.BotWorkflow().GetMe(context.Background())
	if err != nil || user.ID != "123456" || user.AccountType == nil || *user.AccountType != "bot" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	photo, err := client.BotWorkflow().SendMedia(context.Background(), MediaRequest{ChatID: "-1001", Type: socialhub.MediaTypeImage, Media: MediaInput{FileIDOrURL: "photo-file-id"}, Caption: "caption"})
	if err != nil || len(photo.Media) != 1 || photo.Media[0].ID != "large" || photo.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("photo=%#v err=%v", photo, err)
	}
	document, err := client.BotWorkflow().SendMedia(context.Background(), MediaRequest{ChatID: "-1001", Type: socialhub.MediaTypeDocument, Media: MediaInput{Filename: "report.pdf", Reader: strings.NewReader("pdf-data"), Size: 8}})
	if err != nil || len(document.Media) != 1 || document.Media[0].ID != "doc-id" || document.Media[0].MIME != "application/pdf" {
		t.Fatalf("document=%#v err=%v", document, err)
	}
	if err := client.BotWorkflow().ConfigureWebhook(context.Background(), WebhookRegistration{URL: "https://app.example/telegram", AllowedUpdates: []string{"message", "callback_query"}, MaxConnections: 25}); err != nil {
		t.Fatal(err)
	}
	if err := client.BotWorkflow().DeleteWebhook(context.Background(), true); err != nil {
		t.Fatal(err)
	}
}

func parseTelegramForm(t *testing.T, writer http.ResponseWriter, request *http.Request) bool {
	t.Helper()
	if err := request.ParseMultipartForm(2 << 20); err != nil {
		t.Errorf("parse form: %v", err)
		writer.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func TestMediaInputValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "-1001", false)
	tests := []MediaRequest{
		{ChatID: "-1001", Type: socialhub.MediaTypeImage},
		{ChatID: "-1001", Type: socialhub.MediaTypeImage, Media: MediaInput{FileIDOrURL: "id", Reader: strings.NewReader("x")}},
		{ChatID: "-1001", Type: socialhub.MediaTypeImage, Media: MediaInput{Filename: "../photo.jpg", Reader: strings.NewReader("x"), Size: 1}},
		{ChatID: "-1001", Type: socialhub.MediaTypeImage, Media: MediaInput{Filename: "photo.jpg", Reader: strings.NewReader("x"), Size: maxPhotoUpload + 1}},
	}
	for _, input := range tests {
		if _, err := client.BotWorkflow().SendMedia(context.Background(), input); err == nil {
			t.Fatalf("input=%#v should fail", input)
		}
	}
}

func TestUploadCannotExceedDeclaredSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "-1001", false)
	_, err := client.BotWorkflow().SendMedia(context.Background(), MediaRequest{
		ChatID: "-1001", Type: socialhub.MediaTypeDocument,
		Media: MediaInput{Filename: "report.pdf", Reader: strings.NewReader("too-large"), Size: 1},
	})
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}
