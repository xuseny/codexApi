package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/google/uuid"
)

const (
	windsurfSourceUser      = 1
	windsurfSourceSystem    = 2
	windsurfSourceAssistant = 3
	windsurfSourceTool      = 4
)

type windsurfProtoField struct {
	field    int
	wireType int
	value    []byte
	varint   uint64
}

func windsurfEncodeVarint(value uint64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value == 0 {
			out = append(out, b)
			return out
		}
		out = append(out, b|0x80)
	}
}

func windsurfWriteVarintField(field int, value uint64) []byte {
	return bytes.Join([][]byte{
		windsurfEncodeVarint(uint64(field<<3) | 0),
		windsurfEncodeVarint(value),
	}, nil)
}

func windsurfWriteStringField(field int, value string) []byte {
	data := []byte(value)
	return bytes.Join([][]byte{
		windsurfEncodeVarint(uint64(field<<3) | 2),
		windsurfEncodeVarint(uint64(len(data))),
		data,
	}, nil)
}

func windsurfWriteMessageField(field int, msg []byte) []byte {
	if len(msg) == 0 {
		return nil
	}
	return bytes.Join([][]byte{
		windsurfEncodeVarint(uint64(field<<3) | 2),
		windsurfEncodeVarint(uint64(len(msg))),
		msg,
	}, nil)
}

func windsurfEncodeTimestamp() []byte {
	now := time.Now()
	secs := uint64(now.Unix())
	nanos := uint64(now.Nanosecond())
	parts := [][]byte{windsurfWriteVarintField(1, secs)}
	if nanos > 0 {
		parts = append(parts, windsurfWriteVarintField(2, nanos))
	}
	return bytes.Join(parts, nil)
}

func windsurfBuildMetadata(apiKey, sessionID string) []byte {
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	osName := "linux"
	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	}
	hw := "x86_64"
	if runtime.GOARCH == "arm64" {
		hw = "arm64"
	}
	version := getEnvStringLocal("WINDSURF_CLIENT_VERSION", "2.0.67")
	return bytes.Join([][]byte{
		windsurfWriteStringField(1, "windsurf"),
		windsurfWriteStringField(2, version),
		windsurfWriteStringField(3, apiKey),
		windsurfWriteStringField(4, "en"),
		windsurfWriteStringField(5, osName),
		windsurfWriteStringField(7, version),
		windsurfWriteStringField(8, hw),
		windsurfWriteVarintField(9, uint64(rand.Int63n(1<<48))),
		windsurfWriteStringField(10, sessionID),
		windsurfWriteStringField(12, "windsurf"),
	}, nil)
}

func windsurfBuildChatMessage(content string, source int, conversationID string) []byte {
	parts := [][]byte{
		windsurfWriteStringField(1, uuid.NewString()),
		windsurfWriteVarintField(2, uint64(source)),
		windsurfWriteMessageField(3, windsurfEncodeTimestamp()),
		windsurfWriteStringField(4, conversationID),
	}
	if source == windsurfSourceAssistant {
		actionGeneric := windsurfWriteStringField(1, content)
		action := windsurfWriteMessageField(1, actionGeneric)
		parts = append(parts, windsurfWriteMessageField(6, action))
	} else {
		intentGeneric := windsurfWriteStringField(1, content)
		intent := windsurfWriteMessageField(1, intentGeneric)
		parts = append(parts, windsurfWriteMessageField(5, intent))
	}
	return bytes.Join(parts, nil)
}

func windsurfBuildRawGetChatMessageRequest(apiKey string, messages []windsurfRawMessage, modelEnum int, modelName, sessionID string) []byte {
	conversationID := uuid.NewString()
	parts := [][]byte{
		windsurfWriteMessageField(1, windsurfBuildMetadata(apiKey, sessionID)),
	}
	var systemPrompt string
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			if systemPrompt != "" {
				systemPrompt += "\n"
			}
			systemPrompt += msg.Content
			continue
		case "assistant":
			parts = append(parts, windsurfWriteMessageField(2, windsurfBuildChatMessage(msg.Content, windsurfSourceAssistant, conversationID)))
		case "tool", "function":
			parts = append(parts, windsurfWriteMessageField(2, windsurfBuildChatMessage("[tool result]: "+msg.Content, windsurfSourceUser, conversationID)))
		default:
			parts = append(parts, windsurfWriteMessageField(2, windsurfBuildChatMessage(msg.Content, windsurfSourceUser, conversationID)))
		}
	}
	if systemPrompt != "" {
		parts = append(parts, windsurfWriteStringField(3, systemPrompt))
	}
	parts = append(parts, windsurfWriteVarintField(4, uint64(modelEnum)))
	if modelName != "" {
		parts = append(parts, windsurfWriteStringField(5, modelName))
	}
	return bytes.Join(parts, nil)
}

func windsurfParseRawResponse(buf []byte) (string, bool, bool, error) {
	fields, err := windsurfParseFields(buf)
	if err != nil {
		return "", false, false, err
	}
	delta := windsurfGetField(fields, 1, 2)
	if delta == nil {
		return "", false, false, nil
	}
	inner, err := windsurfParseFields(delta.value)
	if err != nil {
		return "", false, false, err
	}
	text := ""
	if f := windsurfGetField(inner, 5, 2); f != nil {
		text = string(f.value)
	}
	inProgress := false
	if f := windsurfGetField(inner, 6, 0); f != nil {
		inProgress = f.varint != 0
	}
	isError := false
	if f := windsurfGetField(inner, 7, 0); f != nil {
		isError = f.varint != 0
	}
	return text, inProgress, isError, nil
}

func windsurfParseFields(buf []byte) ([]windsurfProtoField, error) {
	var fields []windsurfProtoField
	pos := 0
	for pos < len(buf) {
		tag, n, err := windsurfDecodeVarint(buf[pos:])
		if err != nil {
			return nil, err
		}
		pos += n
		fieldNum := int(tag >> 3)
		wireType := int(tag & 7)
		field := windsurfProtoField{field: fieldNum, wireType: wireType}
		switch wireType {
		case 0:
			v, n, err := windsurfDecodeVarint(buf[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
			field.varint = v
		case 1:
			if pos+8 > len(buf) {
				return nil, errors.New("truncated fixed64 field")
			}
			field.value = buf[pos : pos+8]
			pos += 8
		case 2:
			size, n, err := windsurfDecodeVarint(buf[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
			if pos+int(size) > len(buf) {
				return nil, fmt.Errorf("truncated length-delimited field %d", fieldNum)
			}
			field.value = buf[pos : pos+int(size)]
			pos += int(size)
		case 5:
			if pos+4 > len(buf) {
				return nil, errors.New("truncated fixed32 field")
			}
			field.value = buf[pos : pos+4]
			pos += 4
		default:
			return nil, fmt.Errorf("unknown protobuf wire type %d", wireType)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func windsurfDecodeVarint(buf []byte) (uint64, int, error) {
	var value uint64
	for i := 0; i < len(buf) && i < 10; i++ {
		b := buf[i]
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1, nil
		}
	}
	return 0, 0, errors.New("truncated or overflowing varint")
}

func windsurfGetField(fields []windsurfProtoField, field, wireType int) *windsurfProtoField {
	for i := range fields {
		if fields[i].field == field && fields[i].wireType == wireType {
			return &fields[i]
		}
	}
	return nil
}

func windsurfWrapGRPCFrame(payload []byte) []byte {
	frame := make([]byte, len(payload)+5)
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

func windsurfReadGRPCFrames(data []byte) ([][]byte, error) {
	var frames [][]byte
	for len(data) > 0 {
		if len(data) < 5 {
			return frames, errors.New("truncated grpc frame header")
		}
		size := int(binary.BigEndian.Uint32(data[1:5]))
		if len(data) < 5+size {
			return frames, errors.New("truncated grpc frame body")
		}
		frames = append(frames, data[5:5+size])
		data = data[5+size:]
	}
	return frames, nil
}
