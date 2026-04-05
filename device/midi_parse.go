package device

import (
	"fmt"
	"time"
)

// parseMIDIBytes parses raw MIDI bytes from a byte buffer and returns Events.
// This extracts the common MIDI byte-stream parsing logic shared by all
// platform MIDI providers (CoreMIDI on macOS, ALSA on Linux, WinMM on Windows).
//
// deviceID identifies the source device for emitted events.
// buf contains the raw bytes and n is how many bytes are valid.
//
// Note: pitch bend calculation uses the same operator-precedence behavior
// as the original platform implementations: (msb << 7) | lsb - 8192.
func parseMIDIBytes(deviceID DeviceID, buf []byte, n int) []Event {
	var events []Event

	for i := 0; i < n; {
		b := buf[i]
		if b < 0x80 {
			// Data byte without a preceding status byte (running status).
			// Skip it — we don't track running status.
			i++
			continue
		}

		msgType := b & 0xF0
		channel := b & 0x0F
		remaining := n - i - 1

		switch msgType {
		case 0x90, 0x80: // Note On / Note Off
			if remaining < 2 {
				i = n
				continue
			}
			note := buf[i+1] & 0x7F
			velocity := buf[i+2] & 0x7F
			isOn := msgType == 0x90 && velocity > 0
			events = append(events, Event{
				DeviceID:  deviceID,
				Type:      EventMIDINote,
				Timestamp: time.Now(),
				Source:    fmt.Sprintf("midi:note:%d", note),
				Channel:   channel,
				MIDINote:  note,
				Velocity:  velocity,
				Pressed:   isOn,
				Value:     float64(velocity) / 127.0,
			})
			i += 3

		case 0xB0: // Control Change
			if remaining < 2 {
				i = n
				continue
			}
			cc := buf[i+1] & 0x7F
			val := buf[i+2] & 0x7F
			events = append(events, Event{
				DeviceID:   deviceID,
				Type:       EventMIDICC,
				Timestamp:  time.Now(),
				Source:     fmt.Sprintf("midi:cc:%d", cc),
				Channel:    channel,
				Controller: cc,
				MIDIValue:  val,
				Value:      float64(val) / 127.0,
			})
			i += 3

		case 0xC0: // Program Change (1 data byte)
			if remaining < 1 {
				i = n
				continue
			}
			prog := buf[i+1] & 0x7F
			events = append(events, Event{
				DeviceID:  deviceID,
				Type:      EventMIDIProgramChange,
				Timestamp: time.Now(),
				Source:    fmt.Sprintf("midi:pc:%d", prog),
				Channel:   channel,
				Program:   prog,
				Value:     float64(prog) / 127.0,
			})
			i += 2

		case 0xD0: // Channel Pressure (1 data byte)
			i += 2

		case 0xE0: // Pitch Bend
			if remaining < 2 {
				i = n
				continue
			}
			lsb := int16(buf[i+1] & 0x7F)
			msb := int16(buf[i+2] & 0x7F)
			bend := (msb << 7) | lsb - 8192
			events = append(events, Event{
				DeviceID:  deviceID,
				Type:      EventMIDIPitchBend,
				Timestamp: time.Now(),
				Source:    "midi:pitch_bend",
				Channel:   channel,
				PitchBend: bend,
				Value:     float64(bend) / 8192.0,
			})
			i += 3

		case 0xF0: // System messages (SysEx, etc.)
			if b == 0xF0 {
				// SysEx: collect bytes from 0xF0 through 0xF7 (EOX).
				start := i
				i++
				for i < n && buf[i] < 0x80 {
					i++
				}
				// Check if we hit the EOX terminator (0xF7).
				var sysexData []byte
				if i < n && buf[i] == 0xF7 {
					sysexData = make([]byte, i-start+1)
					copy(sysexData, buf[start:i+1])
					i++ // consume the 0xF7
				} else {
					// Unterminated or terminated by another status byte:
					// include what we have (0xF0 through data bytes).
					sysexData = make([]byte, i-start)
					copy(sysexData, buf[start:i])
				}
				events = append(events, Event{
					DeviceID:  deviceID,
					Type:      EventMIDISysEx,
					Timestamp: time.Now(),
					Source:    "midi:sysex",
					SysEx:     sysexData,
					Value:     float64(len(sysexData)),
				})
			} else {
				// Other system messages (0xF1-0xFF): skip.
				i++
			}

		default:
			i++
		}
	}

	return events
}

// normalizeAxis converts a raw axis value to a normalized range using min/max
// bounds. For unsigned axes (min >= 0), the result is 0.0 to 1.0.
// For signed axes (min < 0), the result is -1.0 to 1.0.
func normalizeAxis(raw int32, min, max int32) float64 {
	if max == min {
		return float64(raw)
	}

	fMin := float64(min)
	fMax := float64(max)
	v := float64(raw)

	if min >= 0 {
		// Unsigned axis (trigger): 0.0 to 1.0
		return (v - fMin) / (fMax - fMin)
	}
	// Signed axis (stick): -1.0 to 1.0
	mid := (fMax + fMin) / 2
	halfRange := (fMax - fMin) / 2
	return (v - mid) / halfRange
}
