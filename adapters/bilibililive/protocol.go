package bilibililive

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	protocolHeaderBytes = 16
	maxProtocolBytes    = 8 << 20
	maxProtocolDepth    = 4
	maxPacketsPerFrame  = 4096
)

// Operation is a Bilibili live application-protocol operation number.
type Operation uint32

const (
	OperationHeartbeat      Operation = 2
	OperationHeartbeatReply Operation = 3
	OperationMessage        Operation = 5
	OperationAuth           Operation = 7
	OperationAuthReply      Operation = 8
)

// Packet is one decoded Bilibili live protocol packet. Compressed envelopes
// are expanded, so Body always contains an uncompressed inner payload.
type Packet struct {
	Version   uint16
	Operation Operation
	Sequence  uint32
	Body      []byte
}

// EncodePacket encodes one uncompressed application-protocol packet.
func EncodePacket(operation Operation, sequence uint32, body []byte) ([]byte, error) {
	if len(body) > maxProtocolBytes-protocolHeaderBytes {
		return nil, errors.New("bilibililive: packet body exceeds the protocol limit")
	}
	packet := make([]byte, protocolHeaderBytes+len(body))
	binary.BigEndian.PutUint32(packet[0:4], uint32(len(packet)))
	binary.BigEndian.PutUint16(packet[4:6], protocolHeaderBytes)
	binary.BigEndian.PutUint16(packet[6:8], 0)
	binary.BigEndian.PutUint32(packet[8:12], uint32(operation))
	binary.BigEndian.PutUint32(packet[12:16], sequence)
	copy(packet[protocolHeaderBytes:], body)
	return packet, nil
}

// DecodePackets validates a WebSocket binary frame, recursively expands zlib
// envelopes, and returns independent packet bodies.
func DecodePackets(frame []byte) ([]Packet, error) {
	if len(frame) == 0 || len(frame) > maxProtocolBytes {
		return nil, errors.New("bilibililive: invalid protocol frame size")
	}
	packets := make([]Packet, 0, 4)
	decompressionBudget := maxProtocolBytes
	if err := decodePackets(frame, 0, &decompressionBudget, &packets); err != nil {
		return nil, err
	}
	return packets, nil
}

func decodePackets(frame []byte, depth int, decompressionBudget *int, packets *[]Packet) error {
	if depth > maxProtocolDepth {
		return errors.New("bilibililive: compressed packet nesting exceeded the limit")
	}
	for offset := 0; offset < len(frame); {
		if len(frame)-offset < protocolHeaderBytes {
			return errors.New("bilibililive: truncated protocol header")
		}
		packetLength := int(binary.BigEndian.Uint32(frame[offset : offset+4]))
		headerLength := int(binary.BigEndian.Uint16(frame[offset+4 : offset+6]))
		version := binary.BigEndian.Uint16(frame[offset+6 : offset+8])
		operation := Operation(binary.BigEndian.Uint32(frame[offset+8 : offset+12]))
		sequence := binary.BigEndian.Uint32(frame[offset+12 : offset+16])
		if packetLength < protocolHeaderBytes || packetLength > maxProtocolBytes || packetLength > len(frame)-offset {
			return errors.New("bilibililive: invalid protocol packet length")
		}
		if headerLength < protocolHeaderBytes || headerLength > packetLength {
			return errors.New("bilibililive: invalid protocol header length")
		}
		body := frame[offset+headerLength : offset+packetLength]
		switch version {
		case 0, 1:
			if len(*packets) >= maxPacketsPerFrame {
				return errors.New("bilibililive: packet count exceeded the frame limit")
			}
			*packets = append(*packets, Packet{
				Version: version, Operation: operation, Sequence: sequence, Body: append([]byte(nil), body...),
			})
		case 2:
			decompressed, err := decompressPacketBody(body, *decompressionBudget)
			if err != nil {
				return err
			}
			*decompressionBudget -= len(decompressed)
			if err := decodePackets(decompressed, depth+1, decompressionBudget, packets); err != nil {
				return err
			}
		default:
			return fmt.Errorf("bilibililive: unsupported protocol version %d", version)
		}
		offset += packetLength
	}
	return nil
}

func decompressPacketBody(body []byte, limit int) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("bilibililive: cumulative decompressed data exceeded the protocol limit")
	}
	reader, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bilibililive: open zlib packet: %w", err)
	}
	decompressed, readErr := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("bilibililive: decompress packet: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("bilibililive: close zlib packet: %w", closeErr)
	}
	if len(decompressed) == 0 || len(decompressed) > limit {
		return nil, errors.New("bilibililive: cumulative decompressed data exceeded the protocol limit")
	}
	return decompressed, nil
}

type commandEnvelope struct {
	Command Command         `json:"cmd"`
	Data    json.RawMessage `json:"data"`
}

// DecodeMessage converts a documented message packet into a typed payload.
// Unknown commands remain available as json.RawMessage without data loss.
func DecodeMessage(body []byte) (Message, error) {
	if len(body) == 0 || len(body) > maxProtocolBytes {
		return Message{}, errors.New("bilibililive: invalid command body size")
	}
	var envelope commandEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Message{}, fmt.Errorf("bilibililive: decode command envelope: %w", err)
	}
	if envelope.Command == "" || len(envelope.Data) == 0 || !json.Valid(envelope.Data) {
		return Message{}, errors.New("bilibililive: command or command data is missing")
	}
	data, err := decodeCommandData(envelope.Command, envelope.Data)
	if err != nil {
		return Message{}, err
	}
	rawData := append(json.RawMessage(nil), envelope.Data...)
	return Message{
		Command: envelope.Command, ID: commandMessageID(envelope.Data), Data: data,
		RawData: rawData, Raw: append(json.RawMessage(nil), body...),
	}, nil
}

func decodeCommandData(command Command, raw json.RawMessage) (any, error) {
	var target any
	switch command {
	case CommandDanmaku:
		target = &DanmakuData{}
	case CommandMirrorDanmaku:
		target = &MirrorDanmakuData{}
	case CommandGift:
		target = &GiftData{}
	case CommandSuperChat:
		target = &SuperChatData{}
	case CommandSuperChatDelete:
		target = &SuperChatDeleteData{}
	case CommandGuard:
		target = &GuardData{}
	case CommandLike:
		target = &LikeData{}
	case CommandRoomEnter:
		target = &RoomEnterData{}
	case CommandLiveStart, CommandLiveEnd:
		target = &LiveStatusData{}
	case CommandInteractionEnd:
		target = &InteractionEndData{}
	default:
		return append(json.RawMessage(nil), raw...), nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("bilibililive: decode %s payload: %w", command, err)
	}
	return target, nil
}

func commandMessageID(raw json.RawMessage) string {
	var identifiers struct {
		MessageID json.RawMessage `json:"msg_id"`
		GameID    string          `json:"game_id"`
	}
	if err := json.Unmarshal(raw, &identifiers); err != nil {
		return ""
	}
	if len(identifiers.MessageID) != 0 {
		var text string
		if json.Unmarshal(identifiers.MessageID, &text) == nil && text != "" {
			return text
		}
		var number json.Number
		if json.Unmarshal(identifiers.MessageID, &number) == nil {
			if value := number.String(); value != "" {
				return value
			}
		}
	}
	if identifiers.GameID != "" {
		return identifiers.GameID
	}
	var legacy struct {
		MessageID int64 `json:"message_id"`
	}
	if json.Unmarshal(raw, &legacy) == nil && legacy.MessageID != 0 {
		return strconv.FormatInt(legacy.MessageID, 10)
	}
	return ""
}
