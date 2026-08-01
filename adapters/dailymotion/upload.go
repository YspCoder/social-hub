package dailymotion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxUploadResponseSize int64 = 1 << 20
	maxVideoSize          int64 = 64 << 30
)

type videoUpload struct {
	URL          string
	ProgressURL  string
	Filename     string
	Size         int64
	UploadedURL  string
	UploadedFile UploadedFile
	Busy         bool
}

// VideoUploadService implements VideoUploadWorkflow.
type VideoUploadService struct{ client *Client }

func (s *VideoUploadService) Initialize(ctx context.Context, filename string, size int64, options ...socialhub.CallOption) (*UploadSession, error) {
	if err := s.client.requireScopes("upload_initialize", ScopeVideoManage); err != nil {
		return nil, err
	}
	if !validFilename(filename) || size <= 0 || size > maxVideoSize {
		return nil, invalidArgument("upload_initialize", "a safe filename and size between 1 byte and 64 GiB are required")
	}
	var response uploadSessionResponse
	if err := s.client.requestJSON(ctx, http.MethodPost, "/files/upload_sessions", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validDailymotionUploadURL(response.UploadURL, s.client.apiBaseURL) || !validDailymotionUploadURL(response.ProgressURL, s.client.apiBaseURL) {
		return nil, platformError("upload_initialize", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Dailymotion returned an invalid upload origin"))
	}
	digest := sha256.Sum256([]byte(response.UploadURL + "\x00" + filename))
	sessionID := hex.EncodeToString(digest[:])
	s.client.uploadMu.Lock()
	s.client.uploads[sessionID] = &videoUpload{URL: response.UploadURL, ProgressURL: response.ProgressURL, Filename: filename, Size: size}
	s.client.uploadMu.Unlock()
	return &UploadSession{ID: sessionID, ProgressURL: response.ProgressURL, Filename: filename, Size: size}, nil
}

func (s *VideoUploadService) Upload(ctx context.Context, sessionID string, reader io.Reader, options ...socialhub.CallOption) (*UploadedFile, error) {
	if sessionID == "" || reader == nil {
		return nil, invalidArgument("upload_file", "session ID and reader are required")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_file", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Busy || state.UploadedURL != "" {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_file", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Busy = true
	uploadURL, filename, expectedSize := state.URL, state.Filename, state.Size
	s.client.uploadMu.Unlock()

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pipeReader)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_file", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}

	type copyResult struct {
		bytes int64
		err   error
	}
	copyDone := make(chan copyResult, 1)
	go func() {
		part, createErr := multipartWriter.CreateFormFile("file", filename)
		var count int64
		copyErr := createErr
		if copyErr == nil {
			count, copyErr = io.Copy(part, io.LimitReader(reader, expectedSize+1))
		}
		if closeErr := multipartWriter.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil && count != expectedSize {
			copyErr = &uploadSizeError{expected: expectedSize, actual: count}
		}
		_ = pipeWriter.CloseWithError(copyErr)
		copyDone <- copyResult{bytes: count, err: copyErr}
	}()

	uploadClient := *s.client.httpClient
	uploadClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, requestErr := uploadClient.Do(request)
	if requestErr != nil {
		_ = pipeReader.CloseWithError(requestErr)
		result := <-copyDone
		s.setUploadIdle(sessionID)
		var sizeErr *uploadSizeError
		if errors.As(result.err, &sizeErr) {
			return nil, invalidArgument("upload_file", sizeErr.Error())
		}
		return nil, platformError("upload_file", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, requestErr)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadResponseSize+1))
	// An upload host may reject a request before consuming its body. Closing the
	// read side ensures the multipart writer cannot remain blocked in that case.
	_ = pipeReader.Close()
	result := <-copyDone
	var sizeErr *uploadSizeError
	if errors.As(result.err, &sizeErr) {
		s.setUploadIdle(sessionID)
		return nil, invalidArgument("upload_file", sizeErr.Error())
	}
	if readErr != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_file", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(body)) > maxUploadResponseSize {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_file", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("upload response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.setUploadIdle(sessionID)
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	if result.err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_file", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, result.err)
	}
	var payload uploadFileResponse
	if err := json.Unmarshal(body, &payload); err != nil || !validDailymotionUploadURL(payload.URL, s.client.apiBaseURL) {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_file", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("invalid upload response"))
	}
	uploaded := UploadedFile{URL: payload.URL, Name: payload.Name, Format: payload.Format, Duration: payload.Duration, Size: payload.Size, Checksum: payload.Hash}
	s.client.uploadMu.Lock()
	state = s.client.uploads[sessionID]
	if state == nil || !state.Busy {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_file", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.UploadedURL, state.UploadedFile, state.Busy = payload.URL, uploaded, false
	s.client.uploadMu.Unlock()
	return &uploaded, nil
}

func (s *VideoUploadService) Publish(ctx context.Context, sessionID string, input CreateVideoRequest, options ...socialhub.CallOption) (*Video, error) {
	if sessionID == "" {
		return nil, invalidArgument("upload_publish", "session ID is required")
	}
	if input.SourceURL != "" {
		return nil, invalidArgument("upload_publish", "source URL is supplied by the completed upload session")
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_publish", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Busy || state.UploadedURL == "" {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Busy = true
	input.SourceURL = state.UploadedURL
	s.client.uploadMu.Unlock()
	video, err := s.client.CreateVideo(ctx, input, options...)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, err
	}
	s.client.uploadMu.Lock()
	delete(s.client.uploads, sessionID)
	s.client.uploadMu.Unlock()
	return video, nil
}

func (s *VideoUploadService) Abort(sessionID string) error {
	if sessionID == "" {
		return invalidArgument("upload_abort", "session ID is required")
	}
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	state := s.client.uploads[sessionID]
	if state == nil {
		return platformError("upload_abort", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Busy {
		return platformError("upload_abort", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	delete(s.client.uploads, sessionID)
	return nil
}

func (s *VideoUploadService) setUploadIdle(sessionID string) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[sessionID]; state != nil {
		state.Busy = false
	}
}

func validDailymotionUploadURL(value string, apiBase *url.URL) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "dailymotion.com" || strings.HasSuffix(hostname, ".dailymotion.com") {
		return parsed.Scheme == "https"
	}
	return apiBase != nil && parsed.Scheme == apiBase.Scheme && parsed.Host == apiBase.Host && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

type uploadSizeError struct{ expected, actual int64 }

func (e *uploadSizeError) Error() string {
	return fmt.Sprintf("upload reader size is %d bytes; expected exactly %d", e.actual, e.expected)
}

var _ VideoUploadWorkflow = (*VideoUploadService)(nil)
