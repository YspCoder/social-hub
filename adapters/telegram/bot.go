package telegram

import (
	"context"
	"io"
	"net/url"
	"path"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"social-hub/pkg/socialhub"
)

const (
	maxPhotoUpload = 10 << 20
	maxFileUpload  = 50 << 20
)

// MediaInput identifies either a Telegram file ID/HTTP URL or a multipart upload.
type MediaInput struct {
	FileIDOrURL string
	Filename    string
	Reader      io.Reader
	Size        int64
}

// MediaRequest sends one typed Telegram media message.
type MediaRequest struct {
	ChatID    string
	Type      socialhub.MediaType
	Media     MediaInput
	Caption   string
	ReplyToID string
}

// WebhookRegistration configures Telegram's outgoing webhook delivery.
type WebhookRegistration struct {
	URL                string
	AllowedUpdates     []string
	MaxConnections     int
	DropPendingUpdates bool
}

// BotWorkflow exposes Telegram-specific operations that do not fit common interfaces.
type BotWorkflow interface {
	GetMe(context.Context, ...socialhub.CallOption) (*socialhub.User, error)
	SendMedia(context.Context, MediaRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	ConfigureWebhook(context.Context, WebhookRegistration, ...socialhub.CallOption) error
	DeleteWebhook(context.Context, bool, ...socialhub.CallOption) error
}

// BotService implements BotWorkflow.
type BotService struct{ client *Client }

func (s *BotService) GetMe(ctx context.Context, options ...socialhub.CallOption) (*socialhub.User, error) {
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	user, err := s.client.bot.GetMe(callCtx)
	if err != nil {
		return nil, mapError("get_me", err)
	}
	if user == nil || user.ID == 0 {
		return nil, wrapError("get_me", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := mapUser(s.client.accountID, *user)
	return &result, nil
}

func (s *BotService) SendMedia(ctx context.Context, input MediaRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if input.ChatID == "" {
		return nil, invalidArgument("send_media", "chat ID is required")
	}
	if len([]rune(input.Caption)) > 1024 {
		return nil, invalidArgument("send_media", "caption must not exceed 1024 characters")
	}
	media, err := inputFile(input.Type, input.Media)
	if err != nil {
		return nil, err
	}
	var reply *models.ReplyParameters
	if input.ReplyToID != "" {
		messageID, err := parseMessageID("send_media", input.ReplyToID)
		if err != nil {
			return nil, err
		}
		reply = &models.ReplyParameters{MessageID: messageID}
	}
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var message *models.Message
	switch input.Type {
	case socialhub.MediaTypeImage:
		message, err = s.client.bot.SendPhoto(callCtx, &tgbot.SendPhotoParams{ChatID: input.ChatID, Photo: media, Caption: input.Caption, ReplyParameters: reply})
	case socialhub.MediaTypeVideo:
		message, err = s.client.bot.SendVideo(callCtx, &tgbot.SendVideoParams{ChatID: input.ChatID, Video: media, Caption: input.Caption, ReplyParameters: reply})
	case socialhub.MediaTypeAudio:
		message, err = s.client.bot.SendAudio(callCtx, &tgbot.SendAudioParams{ChatID: input.ChatID, Audio: media, Caption: input.Caption, ReplyParameters: reply})
	case socialhub.MediaTypeDocument:
		message, err = s.client.bot.SendDocument(callCtx, &tgbot.SendDocumentParams{ChatID: input.ChatID, Document: media, Caption: input.Caption, ReplyParameters: reply})
	case socialhub.MediaTypeAnimation:
		message, err = s.client.bot.SendAnimation(callCtx, &tgbot.SendAnimationParams{ChatID: input.ChatID, Animation: media, Caption: input.Caption, ReplyParameters: reply})
	default:
		return nil, unsupported("send_media", "supported media types are image, video, audio, document, and animation")
	}
	if err != nil {
		return nil, mapError("send_media", err)
	}
	if message == nil {
		return nil, wrapError("send_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapMessage(s.client.accountID, message, socialhub.DirectionOutbound), nil
}

func (s *BotService) ConfigureWebhook(ctx context.Context, input WebhookRegistration, options ...socialhub.CallOption) error {
	if s.client.webhookSecret == "" {
		return unsupported("configure_webhook", "webhook.secret_ref is required for verified webhook registration")
	}
	webhookURL, err := url.Parse(input.URL)
	if err != nil || webhookURL.Scheme != "https" || webhookURL.Host == "" || webhookURL.User != nil {
		return invalidArgument("configure_webhook", "webhook URL must be an absolute HTTPS URL without user information")
	}
	if input.MaxConnections < 0 || input.MaxConnections > 100 {
		return invalidArgument("configure_webhook", "max connections must be between 1 and 100 when specified")
	}
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return err
	}
	defer cancel()
	ok, err := s.client.bot.SetWebhook(callCtx, &tgbot.SetWebhookParams{
		URL: input.URL, AllowedUpdates: append([]string(nil), input.AllowedUpdates...), MaxConnections: input.MaxConnections,
		DropPendingUpdates: input.DropPendingUpdates, SecretToken: s.client.webhookSecret,
	})
	if err != nil {
		return mapError("configure_webhook", err)
	}
	if !ok {
		return wrapError("configure_webhook", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (s *BotService) DeleteWebhook(ctx context.Context, dropPending bool, options ...socialhub.CallOption) error {
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return err
	}
	defer cancel()
	ok, err := s.client.bot.DeleteWebhook(callCtx, &tgbot.DeleteWebhookParams{DropPendingUpdates: dropPending})
	if err != nil {
		return mapError("delete_webhook", err)
	}
	if !ok {
		return wrapError("delete_webhook", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func inputFile(mediaType socialhub.MediaType, input MediaInput) (models.InputFile, error) {
	hasReference := strings.TrimSpace(input.FileIDOrURL) != ""
	hasUpload := input.Reader != nil
	if hasReference == hasUpload {
		return nil, invalidArgument("send_media", "exactly one file ID/URL or upload reader is required")
	}
	if hasReference {
		return &models.InputFileString{Data: input.FileIDOrURL}, nil
	}
	if input.Filename == "" || path.Base(input.Filename) != input.Filename || strings.ContainsAny(input.Filename, "\\\r\n") {
		return nil, invalidArgument("send_media", "upload filename must be a safe base name")
	}
	maximum := int64(maxFileUpload)
	if mediaType == socialhub.MediaTypeImage {
		maximum = maxPhotoUpload
	}
	if input.Size <= 0 || input.Size > maximum {
		return nil, invalidArgument("send_media", "upload size must be positive and within the documented media limit")
	}
	return &models.InputFileUpload{Filename: input.Filename, Data: &uploadLimitReader{reader: input.Reader, maximum: input.Size}}, nil
}

type uploadLimitReader struct {
	reader  io.Reader
	maximum int64
	read    int64
}

func (r *uploadLimitReader) Read(buffer []byte) (int, error) {
	remaining := r.maximum - r.read
	if remaining < 0 {
		return 0, errUploadTooLarge
	}
	if int64(len(buffer)) > remaining+1 {
		buffer = buffer[:remaining+1]
	}
	n, err := r.reader.Read(buffer)
	r.read += int64(n)
	if r.read > r.maximum {
		return 0, errUploadTooLarge
	}
	return n, err
}

var _ BotWorkflow = (*BotService)(nil)
