package bilibililive

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"social-hub/pkg/socialhub"
)

const (
	maxAuthBodyBytes         = 64 << 10
	defaultMessageBuffer     = 256
	defaultHeartbeatInterval = 20 * time.Second
	defaultAuthTimeout       = 10 * time.Second
	defaultReconnectMinDelay = time.Second
	defaultReconnectMaxDelay = 30 * time.Second
)

var (
	errMessageBufferFull = errors.New("bilibililive: message buffer is full")
	errInteractionEnded  = errors.New("bilibililive: project message delivery ended")
)

type streamOptions struct {
	messageBuffer     int
	heartbeatInterval time.Duration
	authTimeout       time.Duration
	reconnectMinDelay time.Duration
	reconnectMaxDelay time.Duration
	readLimit         int64
}

// StreamOption configures one live message stream.
type StreamOption func(*streamOptions) error

// WithMessageBuffer sets the bounded number of messages waiting for a reader.
func WithMessageBuffer(size int) StreamOption {
	return func(options *streamOptions) error {
		if size < 1 || size > 65536 {
			return invalidArgument("stream_options", "message buffer must be between 1 and 65536")
		}
		options.messageBuffer = size
		return nil
	}
}

// WithWebSocketHeartbeat sets the application-protocol heartbeat interval.
// Bilibili recommends 20 seconds and disconnects clients after missed heartbeats.
func WithWebSocketHeartbeat(interval time.Duration) StreamOption {
	return func(options *streamOptions) error {
		if interval < 5*time.Second || interval > 25*time.Second {
			return invalidArgument("stream_options", "WebSocket heartbeat must be between 5 and 25 seconds")
		}
		options.heartbeatInterval = interval
		return nil
	}
}

// WithReconnectBackoff controls reconnect delays after an established stream
// is interrupted.
func WithReconnectBackoff(minimum, maximum time.Duration) StreamOption {
	return func(options *streamOptions) error {
		if minimum <= 0 || maximum < minimum || maximum > 5*time.Minute {
			return invalidArgument("stream_options", "reconnect delays must be positive, ordered, and at most five minutes")
		}
		options.reconnectMinDelay, options.reconnectMaxDelay = minimum, maximum
		return nil
	}
}

// WithWebSocketReadLimit sets the WebSocket frame limit. Protocol frames are
// always capped at 8 MiB before and after expansion.
func WithWebSocketReadLimit(limit int64) StreamOption {
	return func(options *streamOptions) error {
		if limit < 64<<10 || limit > maxProtocolBytes {
			return invalidArgument("stream_options", "WebSocket read limit must be between 64 KiB and 8 MiB")
		}
		options.readLimit = limit
		return nil
	}
}

// MessageStream delivers decoded live commands. Errors contains transient
// reconnect notices and closes when Messages closes.
type MessageStream struct {
	client   *Client
	info     WebSocketInfo
	options  streamOptions
	ctx      context.Context
	cancel   context.CancelFunc
	messages chan Message
	errors   chan error
	done     chan struct{}

	connectionMu sync.Mutex
	connection   *websocket.Conn
	nextCluster  int
	closeOnce    sync.Once
}

// Messages returns the bounded decoded-command channel.
func (stream *MessageStream) Messages() <-chan Message { return stream.messages }

// Errors returns reconnect diagnostics and terminal stream errors.
func (stream *MessageStream) Errors() <-chan error { return stream.errors }

// Done closes after all stream resources and output channels are closed.
func (stream *MessageStream) Done() <-chan struct{} { return stream.done }

// Close stops reconnects and releases the active WebSocket connection.
func (stream *MessageStream) Close() error {
	stream.closeOnce.Do(func() {
		stream.cancel()
		stream.connectionMu.Lock()
		if stream.connection != nil {
			_ = stream.connection.CloseNow()
		}
		stream.connectionMu.Unlock()
		<-stream.done
	})
	return nil
}

func (client *Client) ConnectMessages(ctx context.Context, info WebSocketInfo, options ...StreamOption) (*MessageStream, error) {
	if err := client.ensureOpen("connect_messages"); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, invalidArgument("connect_messages", "context must not be nil")
	}
	if !client.issuedWebSocketInfo(info) {
		return nil, invalidArgument("connect_messages", "WebSocket info must be an unchanged value returned by this client")
	}
	normalized, err := normalizeWebSocketInfo(info)
	if err != nil {
		return nil, err
	}
	resolved := streamOptions{
		messageBuffer: defaultMessageBuffer, heartbeatInterval: defaultHeartbeatInterval,
		authTimeout: defaultAuthTimeout, reconnectMinDelay: defaultReconnectMinDelay,
		reconnectMaxDelay: defaultReconnectMaxDelay, readLimit: maxProtocolBytes,
	}
	for _, option := range options {
		if option != nil {
			if err := option(&resolved); err != nil {
				return nil, err
			}
		}
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream := &MessageStream{
		client: client, info: normalized, options: resolved, ctx: streamCtx, cancel: cancel,
		messages: make(chan Message, resolved.messageBuffer), errors: make(chan error, 16), done: make(chan struct{}),
	}
	connection, nextCluster, pending, err := stream.connectCluster(0)
	if err != nil {
		cancel()
		return nil, err
	}
	stream.connection, stream.nextCluster = connection, nextCluster
	if err := client.addStream(stream); err != nil {
		cancel()
		_ = connection.CloseNow()
		return nil, err
	}
	go stream.run(pending)
	return stream, nil
}

func normalizeWebSocketInfo(info WebSocketInfo) (WebSocketInfo, error) {
	if len(info.AuthBody) == 0 || len(info.AuthBody) > maxAuthBodyBytes || !json.Valid([]byte(info.AuthBody)) {
		return WebSocketInfo{}, invalidArgument("connect_messages", "auth_body must be valid bounded JSON returned by StartProject")
	}
	if len(info.Links) == 0 || len(info.Links) > 16 {
		return WebSocketInfo{}, invalidArgument("connect_messages", "wss_link must contain between 1 and 16 cluster URLs")
	}
	links := make([]string, 0, len(info.Links))
	for _, value := range info.Links {
		value = strings.TrimSpace(value)
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "wss" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || len(value) > 2048 {
			return WebSocketInfo{}, invalidArgument("connect_messages", "every cluster link must be a bounded absolute wss URL without credentials or fragment")
		}
		links = append(links, parsed.String())
	}
	return WebSocketInfo{AuthBody: info.AuthBody, Links: links}, nil
}

func (client *Client) issueWebSocketInfo(info WebSocketInfo) WebSocketInfo {
	info.client = client
	info.fingerprint = fingerprintWebSocketInfo(info)
	return info
}

func (client *Client) issuedWebSocketInfo(info WebSocketInfo) bool {
	return info.client == client && info.fingerprint == fingerprintWebSocketInfo(info)
}

func fingerprintWebSocketInfo(info WebSocketInfo) [32]byte {
	encoded, _ := json.Marshal(struct {
		AuthBody string   `json:"auth_body"`
		Links    []string `json:"wss_link"`
	}{AuthBody: info.AuthBody, Links: info.Links})
	return sha256.Sum256(encoded)
}

func (stream *MessageStream) run(pending []Message) {
	defer func() {
		stream.connectionMu.Lock()
		if stream.connection != nil {
			_ = stream.connection.CloseNow()
			stream.connection = nil
		}
		stream.connectionMu.Unlock()
		stream.client.removeStream(stream)
		close(stream.messages)
		close(stream.errors)
		close(stream.done)
	}()
	for _, message := range pending {
		if !stream.emit(message) {
			stream.report(platformError("message_stream", socialhub.CodeRateLimited, socialhub.ClassUserAction, errMessageBufferFull))
			return
		}
		if message.Command == CommandInteractionEnd {
			return
		}
	}

	backoff := stream.options.reconnectMinDelay
	for {
		connection := stream.currentConnection()
		readErrors := make(chan error, 1)
		go stream.readConnection(connection, readErrors)
		heartbeats := time.NewTicker(stream.options.heartbeatInterval)
		var connectionError error
	selectLoop:
		for {
			select {
			case <-stream.ctx.Done():
				heartbeats.Stop()
				return
			case connectionError = <-readErrors:
				break selectLoop
			case <-heartbeats.C:
				packet, _ := EncodePacket(OperationHeartbeat, 1, nil)
				writeCtx, cancel := context.WithTimeout(stream.ctx, 5*time.Second)
				writeErr := connection.Write(writeCtx, websocket.MessageBinary, packet)
				cancel()
				if writeErr != nil {
					connectionError = writeErr
					break selectLoop
				}
			}
		}
		heartbeats.Stop()
		_ = connection.CloseNow()
		stream.clearConnection(connection)
		if errors.Is(connectionError, errInteractionEnded) {
			return
		}
		if errors.Is(connectionError, errMessageBufferFull) {
			stream.report(platformError("message_stream", socialhub.CodeRateLimited, socialhub.ClassUserAction, errMessageBufferFull))
			return
		}
		if stream.ctx.Err() != nil {
			return
		}
		stream.report(unavailable("message_stream", "WebSocket connection interrupted; reconnecting through configured clusters"))

		for {
			connection, nextCluster, queued, err := stream.connectCluster(stream.nextCluster)
			if err == nil {
				stream.setConnection(connection)
				stream.nextCluster = nextCluster
				for _, message := range queued {
					if !stream.emit(message) {
						stream.report(platformError("message_stream", socialhub.CodeRateLimited, socialhub.ClassUserAction, errMessageBufferFull))
						return
					}
					if message.Command == CommandInteractionEnd {
						return
					}
				}
				backoff = stream.options.reconnectMinDelay
				break
			}
			var platform *socialhub.Error
			if errors.As(err, &platform) && platform.Class != socialhub.ClassRetryable {
				stream.report(err)
				return
			}
			stream.report(err)
			timer := time.NewTimer(backoff)
			select {
			case <-stream.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			backoff = minDuration(backoff*2, stream.options.reconnectMaxDelay)
		}
	}
}

func (stream *MessageStream) readConnection(connection *websocket.Conn, result chan<- error) {
	for {
		messageType, frame, err := connection.Read(stream.ctx)
		if err != nil {
			result <- err
			return
		}
		if messageType != websocket.MessageBinary {
			result <- errors.New("bilibililive: expected a binary WebSocket frame")
			return
		}
		packets, err := DecodePackets(frame)
		if err != nil {
			result <- err
			return
		}
		for _, packet := range packets {
			switch packet.Operation {
			case OperationHeartbeatReply:
				continue
			case OperationAuthReply:
				if err := validateAuthReply(packet.Body); err != nil {
					result <- err
					return
				}
			case OperationMessage:
				message, err := DecodeMessage(packet.Body)
				if err != nil {
					result <- err
					return
				}
				if !stream.emit(message) {
					result <- errMessageBufferFull
					return
				}
				if message.Command == CommandInteractionEnd {
					result <- errInteractionEnded
					return
				}
			}
		}
	}
}

func (stream *MessageStream) connectCluster(start int) (*websocket.Conn, int, []Message, error) {
	var permanent error
	for offset := range len(stream.info.Links) {
		index := (start + offset) % len(stream.info.Links)
		link := stream.info.Links[index]
		connectCtx, cancel := context.WithTimeout(stream.ctx, stream.options.authTimeout)
		connection, _, err := websocket.Dial(connectCtx, link, &websocket.DialOptions{
			HTTPClient: stream.client.httpClient, CompressionMode: websocket.CompressionDisabled,
		})
		if err == nil {
			connection.SetReadLimit(stream.options.readLimit)
			var pending []Message
			pending, err = authenticateConnection(connectCtx, connection, stream.info.AuthBody, stream.options.messageBuffer)
			if err == nil {
				cancel()
				return connection, (index + 1) % len(stream.info.Links), pending, nil
			}
			_ = connection.CloseNow()
		}
		cancel()
		stream.client.logger.WarnContext(stream.ctx, "bilibili live websocket cluster unavailable", "host", websocketHost(link))
		var platform *socialhub.Error
		if errors.As(err, &platform) && platform.Class != socialhub.ClassRetryable {
			permanent = err
		}
	}
	if permanent != nil {
		return nil, start, nil, permanent
	}
	return nil, start, nil, unavailable("connect_messages", "all Bilibili WebSocket clusters failed")
}

func authenticateConnection(ctx context.Context, connection *websocket.Conn, authBody string, pendingLimit int) ([]Message, error) {
	packet, err := EncodePacket(OperationAuth, 1, []byte(authBody))
	if err != nil {
		return nil, err
	}
	if err := connection.Write(ctx, websocket.MessageBinary, packet); err != nil {
		return nil, unavailable("websocket_auth", "failed to send the WebSocket authentication packet")
	}
	pending := make([]Message, 0, 2)
	for range 8 {
		messageType, frame, err := connection.Read(ctx)
		if err != nil {
			return nil, unavailable("websocket_auth", "failed while waiting for the WebSocket authentication reply")
		}
		if messageType != websocket.MessageBinary {
			return nil, platformError("websocket_auth", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		packets, err := DecodePackets(frame)
		if err != nil {
			return nil, platformError("websocket_auth", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		authenticated := false
		for _, current := range packets {
			switch current.Operation {
			case OperationAuthReply:
				if err := validateAuthReply(current.Body); err != nil {
					return nil, err
				}
				authenticated = true
			case OperationMessage:
				message, err := DecodeMessage(current.Body)
				if err != nil {
					return nil, platformError("websocket_auth", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
				}
				if len(pending) >= pendingLimit {
					return nil, platformError("websocket_auth", socialhub.CodeRateLimited, socialhub.ClassUserAction, errMessageBufferFull)
				}
				pending = append(pending, message)
			}
		}
		if authenticated {
			return pending, nil
		}
	}
	return nil, platformError("websocket_auth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
}

func validateAuthReply(body []byte) error {
	var reply struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if len(body) == 0 || json.Unmarshal(body, &reply) != nil {
		return platformError("websocket_auth", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if reply.Code != 0 {
		return &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "bilibili", Product: productName,
			Op: "websocket_auth", PlatformCode: fmt.Sprint(reply.Code), PlatformMessage: boundedMessage(reply.Message, 512),
		}
	}
	return nil
}

func (stream *MessageStream) emit(message Message) bool {
	select {
	case stream.messages <- message:
		return true
	case <-stream.ctx.Done():
		return false
	default:
		return false
	}
}

func (stream *MessageStream) report(err error) {
	if err == nil {
		return
	}
	select {
	case stream.errors <- err:
	default:
	}
}

func (stream *MessageStream) currentConnection() *websocket.Conn {
	stream.connectionMu.Lock()
	defer stream.connectionMu.Unlock()
	return stream.connection
}

func (stream *MessageStream) setConnection(connection *websocket.Conn) {
	stream.connectionMu.Lock()
	stream.connection = connection
	stream.connectionMu.Unlock()
}

func (stream *MessageStream) clearConnection(connection *websocket.Conn) {
	stream.connectionMu.Lock()
	if stream.connection == connection {
		stream.connection = nil
	}
	stream.connectionMu.Unlock()
}

func websocketHost(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
