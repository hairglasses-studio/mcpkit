package device

import (
	"bytes"
	"fmt"
)

// Grid Protocol codec for Intech Studio Grid modular controllers.
//
// The Grid Protocol is a text-based serial protocol that sends Lua commands
// as ASCII text framed with STX/ETX control characters. Communication is
// over USB CDC serial (macOS: /dev/cu.usbmodem*, Linux: /dev/ttyACM*).
//
// All Grid module variants (EN16, EF44, PO16, BU16, PBF4, TEK2) share the
// same VID/PID and protocol — module type is identified via hardware config.

// Grid Protocol framing bytes.
const (
	gridSTX = 0x02 // Start of Text
	gridETX = 0x03 // End of Text
	gridEOT = 0x04 // End of Transmission
	gridACK = 0x06 // Acknowledge
	gridNAK = 0x15 // Negative Acknowledge
	gridEOB = 0x17 // End of Block
)

// Grid USB identifiers (all modules share the same VID/PID).
const (
	GridVIDGen2 uint16 = 0x303A // Espressif (ESP32-S3, current production)
	GridPIDGen2 uint16 = 0x8123
	GridVIDGen1 uint16 = 0x03EB // Atmel (SAMD51, legacy)
	GridPIDGen1 uint16 = 0xECAD
)

// GridSerialBaud is the baud rate for Grid CDC serial communication.
const GridSerialBaud = 115200

// GridLEDLayer selects which LED layer to control.
// Each element has two independent LED layers.
type GridLEDLayer int

const (
	GridLEDLayer1 GridLEDLayer = 1
	GridLEDLayer2 GridLEDLayer = 2
)

// GridAnimationType controls LED animation shape.
type GridAnimationType int

const (
	GridAnimNone     GridAnimationType = 0
	GridAnimRampUp   GridAnimationType = 1
	GridAnimReversed GridAnimationType = 2
	GridAnimSquare   GridAnimationType = 3
	GridAnimSine     GridAnimationType = 4
)

// GridMessage represents a parsed Grid Protocol message.
type GridMessage struct {
	Raw     []byte // Complete frame including STX/ETX
	Payload string // Text content between STX and ETX
}

// ---------------------------------------------------------------------------
// Encoding — build Grid Protocol frames from Lua commands
// ---------------------------------------------------------------------------

// GridEncodeFrame wraps a Lua command string in STX/ETX framing.
func GridEncodeFrame(luaCode string) []byte {
	buf := make([]byte, 0, len(luaCode)+2)
	buf = append(buf, gridSTX)
	buf = append(buf, []byte(luaCode)...)
	buf = append(buf, gridETX)
	return buf
}

// GridEncodeLEDColor sets an LED's color on the specified layer.
// index: element index (0-15 for EN16), layer: 1 or 2, r/g/b: 0-255.
func GridEncodeLEDColor(index int, layer GridLEDLayer, r, g, b uint8) []byte {
	cmd := fmt.Sprintf("led_color(%d,%d,%d,%d,%d,0)", index, layer, r, g, b)
	return GridEncodeFrame(cmd)
}

// GridEncodeLEDColorMin sets the minimum-value color for an LED.
func GridEncodeLEDColorMin(index int, layer GridLEDLayer, r, g, b uint8) []byte {
	cmd := fmt.Sprintf("led_color_min(%d,%d,%d,%d,%d)", index, layer, r, g, b)
	return GridEncodeFrame(cmd)
}

// GridEncodeLEDColorMid sets the mid-value color for an LED.
func GridEncodeLEDColorMid(index int, layer GridLEDLayer, r, g, b uint8) []byte {
	cmd := fmt.Sprintf("led_color_mid(%d,%d,%d,%d,%d)", index, layer, r, g, b)
	return GridEncodeFrame(cmd)
}

// GridEncodeLEDColorMax sets the maximum-value color for an LED.
func GridEncodeLEDColorMax(index int, layer GridLEDLayer, r, g, b uint8) []byte {
	cmd := fmt.Sprintf("led_color_max(%d,%d,%d,%d,%d)", index, layer, r, g, b)
	return GridEncodeFrame(cmd)
}

// GridEncodeLEDValue sets LED intensity (0-255).
func GridEncodeLEDValue(index int, layer GridLEDLayer, value uint8) []byte {
	cmd := fmt.Sprintf("led_value(%d,%d,%d)", index, layer, value)
	return GridEncodeFrame(cmd)
}

// GridEncodeLEDAnimation sets the LED animation parameters.
// rate: -255 to 255 (0 = static), animType: animation shape.
// Use rate=0, animType=0 to stop animation.
func GridEncodeLEDAnimation(index int, layer GridLEDLayer, phase, rate int, animType GridAnimationType) []byte {
	cmd := fmt.Sprintf("led_animation_phase_rate_type(%d,%d,%d,%d,%d)", index, layer, phase, rate, animType)
	return GridEncodeFrame(cmd)
}

// GridEncodeHeartbeat creates a heartbeat/ping frame.
func GridEncodeHeartbeat() []byte {
	return GridEncodeFrame("")
}

// ---------------------------------------------------------------------------
// Decoding — parse incoming Grid Protocol frames
// ---------------------------------------------------------------------------

// GridParseFrames extracts complete STX/ETX-framed messages from a byte buffer.
// Returns parsed messages and any remaining (incomplete) bytes.
func GridParseFrames(data []byte) ([]GridMessage, []byte) {
	var msgs []GridMessage
	for {
		start := bytes.IndexByte(data, gridSTX)
		if start == -1 {
			break
		}
		end := bytes.IndexByte(data[start+1:], gridETX)
		if end == -1 {
			// Incomplete frame — return remainder.
			return msgs, data[start:]
		}
		end += start + 1 // Adjust to absolute index.

		frame := data[start : end+1]
		payload := string(data[start+1 : end])
		msgs = append(msgs, GridMessage{
			Raw:     append([]byte(nil), frame...), // Copy to avoid alias.
			Payload: payload,
		})
		data = data[end+1:]
	}
	// Discard bytes before any STX (garbage).
	return msgs, nil
}

// IsGridDevice checks whether a VID/PID pair matches a known Grid controller.
func IsGridDevice(vid, pid uint16) bool {
	return (vid == GridVIDGen2 && pid == GridPIDGen2) ||
		(vid == GridVIDGen1 && pid == GridPIDGen1)
}

// IsGridDeviceName checks whether a device name matches Grid naming patterns.
func IsGridDeviceName(name string) bool {
	lower := bytes.ToLower([]byte(name))
	return bytes.Contains(lower, []byte("grid")) &&
		bytes.Contains(lower, []byte("cdc"))
}
