package flickr

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

const maxUploadResponseSize int64 = 1 << 20

// PhotoUploadService implements PhotoUploadWorkflow.
type PhotoUploadService struct{ client *Client }

func (service *PhotoUploadService) Upload(ctx context.Context, input UploadPhotoRequest, reader io.Reader, options ...socialhub.CallOption) (*UploadResult, error) {
	if err := service.client.requirePermission("upload_photo", PermissionWrite); err != nil {
		return nil, err
	}
	if err := validateUpload(input, reader != nil); err != nil {
		return nil, err
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, resolved.Timeout)
		defer cancel()
	}
	fields := uploadFields(service.client.apiKey, input)
	authorization, err := service.client.uploadAuthorizationHeader(ctx, fields)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, service.client.uploadURL, pipeReader)
	if err != nil {
		return nil, platformError("upload_photo", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/xml")
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if resolved.RequestID != "" {
		request.Header.Set("X-Request-ID", resolved.RequestID)
	}

	type copyResult struct {
		bytes int64
		err   error
	}
	copyDone := make(chan copyResult, 1)
	go func() {
		var copyErr error
		for _, key := range sortedValueKeys(fields) {
			if copyErr == nil {
				copyErr = multipartWriter.WriteField(key, fields.Get(key))
			}
		}
		var count int64
		if copyErr == nil {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "photo", "filename": input.Filename}))
			header.Set("Content-Type", input.MIME)
			var part io.Writer
			part, copyErr = multipartWriter.CreatePart(header)
			if copyErr == nil {
				count, copyErr = io.Copy(part, io.LimitReader(reader, input.Size+1))
			}
		}
		if closeErr := multipartWriter.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil && count != input.Size {
			copyErr = &uploadSizeError{expected: input.Size, actual: count}
		}
		_ = pipeWriter.CloseWithError(copyErr)
		copyDone <- copyResult{bytes: count, err: copyErr}
	}()

	response, requestErr := service.client.public.Do(request)
	if requestErr != nil {
		_ = pipeReader.CloseWithError(requestErr)
		result := <-copyDone
		var sizeErr *uploadSizeError
		if errors.As(result.err, &sizeErr) {
			return nil, invalidArgument("upload_photo", sizeErr.Error())
		}
		return nil, platformError("upload_photo", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, requestErr)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadResponseSize+1))
	_ = pipeReader.Close()
	result := <-copyDone
	var sizeErr *uploadSizeError
	if errors.As(result.err, &sizeErr) {
		return nil, invalidArgument("upload_photo", sizeErr.Error())
	}
	if readErr != nil {
		return nil, platformError("upload_photo", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(body)) > maxUploadResponseSize {
		return nil, platformError("upload_photo", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Flickr upload response exceeded size limit"))
	}
	var envelope uploadEnvelope
	parseErr := xml.Unmarshal(body, &envelope)
	if response.StatusCode < 200 || response.StatusCode >= 300 || parseErr == nil && envelope.Stat != "ok" {
		return nil, decodeUploadError(response.StatusCode, response.Header, envelope, parseErr)
	}
	if result.err != nil {
		return nil, platformError("upload_photo", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, result.err)
	}
	if parseErr != nil || !validResourceID(strings.TrimSpace(envelope.PhotoID)) {
		return nil, platformError("upload_photo", socialhub.CodePlatformError, socialhub.ClassPermanent, firstError(parseErr, errors.New("Flickr upload response is missing photoid")))
	}
	return &UploadResult{PhotoID: strings.TrimSpace(envelope.PhotoID)}, nil
}

func (c *Client) uploadAuthorizationHeader(ctx context.Context, fields url.Values) (string, error) {
	var authorization string
	capture := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		authorization = request.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})
	baseClient := &http.Client{Transport: capture}
	oauthContext := context.WithValue(ctx, oauth1.HTTPClient, baseClient)
	config := &oauth1.Config{ConsumerKey: c.apiKey, ConsumerSecret: c.consumerSecret}
	signer := config.Client(oauthContext, oauth1.NewToken(c.accessToken, c.tokenSecret))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.uploadURL, strings.NewReader(fields.Encode()))
	if err != nil {
		return "", platformError("upload_sign", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := signer.Do(request)
	if err != nil {
		return "", platformError("upload_sign", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	_ = response.Body.Close()
	if !strings.HasPrefix(authorization, "OAuth ") {
		return "", platformError("upload_sign", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("OAuth1 signer did not produce an authorization header"))
	}
	return authorization, nil
}

func uploadFields(apiKey string, input UploadPhotoRequest) url.Values {
	values := url.Values{"api_key": {apiKey}}
	if input.Title != "" {
		values.Set("title", input.Title)
	}
	if input.Description != "" {
		values.Set("description", input.Description)
	}
	if len(input.Tags) > 0 {
		values.Set("tags", encodeTags(input.Tags))
	}
	setOptionalBool(values, "is_public", input.IsPublic)
	setOptionalBool(values, "is_friend", input.IsFriend)
	setOptionalBool(values, "is_family", input.IsFamily)
	if input.SafetyLevel > 0 {
		values.Set("safety_level", strconv.Itoa(input.SafetyLevel))
	}
	if input.ContentType > 0 {
		values.Set("content_type", strconv.Itoa(input.ContentType))
	}
	if input.Hidden > 0 {
		values.Set("hidden", strconv.Itoa(input.Hidden))
	}
	return values
}

func setOptionalBool(values url.Values, key string, value *bool) {
	if value == nil {
		return
	}
	if *value {
		values.Set(key, "1")
	} else {
		values.Set(key, "0")
	}
}

func encodeTags(tags []string) string {
	encoded := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.ContainsAny(tag, " \t\"") {
			encoded = append(encoded, strconv.Quote(tag))
		} else {
			encoded = append(encoded, tag)
		}
	}
	return strings.Join(encoded, " ")
}

func sortedValueKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type uploadEnvelope struct {
	XMLName xml.Name `xml:"rsp"`
	Stat    string   `xml:"stat,attr"`
	PhotoID string   `xml:"photoid"`
	Error   struct {
		Code int    `xml:"code,attr"`
		Msg  string `xml:"msg,attr"`
	} `xml:"err"`
}

func decodeUploadError(status int, header http.Header, envelope uploadEnvelope, parseErr error) error {
	code, class := classifyUploadError(status, envelope.Error.Code)
	if envelope.Error.Code == 0 && parseErr == nil && envelope.Error.Msg != "" {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	message := envelope.Error.Msg
	if message == "" && parseErr != nil {
		message = parseErr.Error()
	}
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "flickr", Product: productName, Op: "upload_photo",
		HTTPStatus: status, PlatformMessage: boundedMessage(message, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 512),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	if envelope.Error.Code != 0 {
		err.PlatformCode = strconv.Itoa(envelope.Error.Code)
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://www.flickr.com/services/apps/create/"
	}
	return err
}

func classifyUploadError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 14:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 2, 4, 5, 8, 10, 12, 15:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 3:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	case 6, 7:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 9:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 11:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 13:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	default:
		return classifyError(status, platformCode)
	}
}

type uploadSizeError struct{ expected, actual int64 }

func (err *uploadSizeError) Error() string {
	return fmt.Sprintf("upload reader size is %d bytes; expected exactly %d", err.actual, err.expected)
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

var _ PhotoUploadWorkflow = (*PhotoUploadService)(nil)
