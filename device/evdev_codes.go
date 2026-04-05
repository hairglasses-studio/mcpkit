//go:build linux

package device

// evdev event type constants from linux/input-event-codes.h.
const (
	evSyn = 0x00 // sync
	evKey = 0x01 // key/button
	evRel = 0x02 // relative axis
	evAbs = 0x03 // absolute axis
	evMsc = 0x04 // misc
)

// evKeyName maps EV_KEY codes to canonical KEY_*/BTN_* names.
// These are the stable Linux ABI codes from input-event-codes.h.
func evKeyName(code uint16) string {
	if name, ok := evKeyNames[code]; ok {
		return name
	}
	return ""
}

// evAbsName maps EV_ABS codes to canonical ABS_* names.
func evAbsName(code uint16) string {
	if name, ok := evAbsNames[code]; ok {
		return name
	}
	return ""
}

// evRelName maps EV_REL codes to canonical REL_* names.
func evRelName(code uint16) string {
	if name, ok := evRelNames[code]; ok {
		return name
	}
	return ""
}

var evKeyNames = map[uint16]string{
	// Standard keyboard keys
	1: "KEY_ESC", 2: "KEY_1", 3: "KEY_2", 4: "KEY_3", 5: "KEY_4",
	6: "KEY_5", 7: "KEY_6", 8: "KEY_7", 9: "KEY_8", 10: "KEY_9",
	11: "KEY_0", 12: "KEY_MINUS", 13: "KEY_EQUAL", 14: "KEY_BACKSPACE",
	15: "KEY_TAB", 16: "KEY_Q", 17: "KEY_W", 18: "KEY_E", 19: "KEY_R",
	20: "KEY_T", 21: "KEY_Y", 22: "KEY_U", 23: "KEY_I", 24: "KEY_O",
	25: "KEY_P", 26: "KEY_LEFTBRACE", 27: "KEY_RIGHTBRACE", 28: "KEY_ENTER",
	29: "KEY_LEFTCTRL", 30: "KEY_A", 31: "KEY_S", 32: "KEY_D", 33: "KEY_F",
	34: "KEY_G", 35: "KEY_H", 36: "KEY_J", 37: "KEY_K", 38: "KEY_L",
	39: "KEY_SEMICOLON", 40: "KEY_APOSTROPHE", 41: "KEY_GRAVE",
	42: "KEY_LEFTSHIFT", 43: "KEY_BACKSLASH", 44: "KEY_Z", 45: "KEY_X",
	46: "KEY_C", 47: "KEY_V", 48: "KEY_B", 49: "KEY_N", 50: "KEY_M",
	51: "KEY_COMMA", 52: "KEY_DOT", 53: "KEY_SLASH", 54: "KEY_RIGHTSHIFT",
	55: "KEY_KPASTERISK", 56: "KEY_LEFTALT", 57: "KEY_SPACE",
	58: "KEY_CAPSLOCK", 59: "KEY_F1", 60: "KEY_F2", 61: "KEY_F3",
	62: "KEY_F4", 63: "KEY_F5", 64: "KEY_F6", 65: "KEY_F7", 66: "KEY_F8",
	67: "KEY_F9", 68: "KEY_F10", 69: "KEY_NUMLOCK", 70: "KEY_SCROLLLOCK",
	71: "KEY_KP7", 72: "KEY_KP8", 73: "KEY_KP9", 74: "KEY_KPMINUS",
	75: "KEY_KP4", 76: "KEY_KP5", 77: "KEY_KP6", 78: "KEY_KPPLUS",
	79: "KEY_KP1", 80: "KEY_KP2", 81: "KEY_KP3", 82: "KEY_KP0",
	83: "KEY_KPDOT", 87: "KEY_F11", 88: "KEY_F12",
	96: "KEY_KPENTER", 97: "KEY_RIGHTCTRL", 98: "KEY_KPSLASH",
	99: "KEY_SYSRQ", 100: "KEY_RIGHTALT", 102: "KEY_HOME",
	103: "KEY_UP", 104: "KEY_PAGEUP", 105: "KEY_LEFT", 106: "KEY_RIGHT",
	107: "KEY_END", 108: "KEY_DOWN", 109: "KEY_PAGEDOWN",
	110: "KEY_INSERT", 111: "KEY_DELETE", 113: "KEY_MUTE",
	114: "KEY_VOLUMEDOWN", 115: "KEY_VOLUMEUP", 116: "KEY_POWER",
	117: "KEY_KPEQUAL", 119: "KEY_PAUSE",
	121: "KEY_KPCOMMA", 125: "KEY_LEFTMETA", 126: "KEY_RIGHTMETA",
	127: "KEY_COMPOSE",

	// Media / misc
	128: "KEY_STOP", 140: "KEY_CALC", 142: "KEY_SLEEP",
	150: "KEY_WWW", 152: "KEY_SCREENLOCK",
	158: "KEY_BACK", 159: "KEY_FORWARD",
	161: "KEY_EJECTCD", 163: "KEY_NEXTSONG", 164: "KEY_PLAYPAUSE",
	165: "KEY_PREVIOUSSONG", 166: "KEY_STOPCD", 167: "KEY_RECORD",
	168: "KEY_REWIND", 172: "KEY_HOMEPAGE", 173: "KEY_REFRESH",
	183: "KEY_F13", 184: "KEY_F14", 185: "KEY_F15", 186: "KEY_F16",
	187: "KEY_F17", 188: "KEY_F18", 189: "KEY_F19", 190: "KEY_F20",
	191: "KEY_F21", 192: "KEY_F22", 193: "KEY_F23", 194: "KEY_F24",
	210: "KEY_PRINT",
	224: "KEY_BRIGHTNESSDOWN", 225: "KEY_BRIGHTNESSUP",
	248: "KEY_MICMUTE",

	// Mouse buttons
	0x110: "BTN_LEFT", 0x111: "BTN_RIGHT", 0x112: "BTN_MIDDLE",
	0x113: "BTN_SIDE", 0x114: "BTN_EXTRA",
	0x115: "BTN_FORWARD", 0x116: "BTN_BACK", 0x117: "BTN_TASK",

	// Joystick buttons
	0x120: "BTN_TRIGGER", 0x121: "BTN_THUMB", 0x122: "BTN_THUMB2",
	0x123: "BTN_TOP", 0x124: "BTN_TOP2", 0x125: "BTN_PINKIE",
	0x126: "BTN_BASE", 0x127: "BTN_BASE2", 0x128: "BTN_BASE3",
	0x129: "BTN_BASE4", 0x12a: "BTN_BASE5", 0x12b: "BTN_BASE6",
	0x12f: "BTN_DEAD",

	// Gamepad buttons (the primary ones for controller mapping)
	0x130: "BTN_SOUTH", 0x131: "BTN_EAST", 0x132: "BTN_C",
	0x133: "BTN_NORTH", 0x134: "BTN_WEST", 0x135: "BTN_Z",
	0x136: "BTN_TL", 0x137: "BTN_TR", 0x138: "BTN_TL2", 0x139: "BTN_TR2",
	0x13a: "BTN_SELECT", 0x13b: "BTN_START", 0x13c: "BTN_MODE",
	0x13d: "BTN_THUMBL", 0x13e: "BTN_THUMBR",

	// D-pad as buttons (some controllers report this way)
	0x220: "BTN_DPAD_UP", 0x221: "BTN_DPAD_DOWN",
	0x222: "BTN_DPAD_LEFT", 0x223: "BTN_DPAD_RIGHT",

	// Touchpad
	0x14a: "BTN_TOUCH", 0x14b: "BTN_STYLUS", 0x14c: "BTN_STYLUS2",
	0x140: "BTN_TOOL_PEN", 0x141: "BTN_TOOL_RUBBER",
	0x145: "BTN_TOOL_FINGER", 0x14d: "BTN_TOOL_DOUBLETAP",
	0x14e: "BTN_TOOL_TRIPLETAP",
}

var evAbsNames = map[uint16]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x06: "ABS_THROTTLE", 0x07: "ABS_RUDDER",
	0x08: "ABS_WHEEL", 0x09: "ABS_GAS", 0x0a: "ABS_BRAKE",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
	0x12: "ABS_HAT1X", 0x13: "ABS_HAT1Y",
	0x14: "ABS_HAT2X", 0x15: "ABS_HAT2Y",
	0x16: "ABS_HAT3X", 0x17: "ABS_HAT3Y",
	0x18: "ABS_PRESSURE", 0x19: "ABS_DISTANCE",
	0x1a: "ABS_TILT_X", 0x1b: "ABS_TILT_Y",
	0x1c: "ABS_TOOL_WIDTH",
	0x20: "ABS_VOLUME",
	0x28: "ABS_MISC",
	// Multi-touch
	0x2f: "ABS_MT_SLOT", 0x30: "ABS_MT_TOUCH_MAJOR",
	0x31: "ABS_MT_TOUCH_MINOR", 0x32: "ABS_MT_WIDTH_MAJOR",
	0x33: "ABS_MT_WIDTH_MINOR", 0x34: "ABS_MT_ORIENTATION",
	0x35: "ABS_MT_POSITION_X", 0x36: "ABS_MT_POSITION_Y",
	0x37: "ABS_MT_TOOL_TYPE", 0x38: "ABS_MT_BLOB_ID",
	0x39: "ABS_MT_TRACKING_ID", 0x3a: "ABS_MT_PRESSURE",
	0x3b: "ABS_MT_DISTANCE", 0x3c: "ABS_MT_TOOL_X",
	0x3d: "ABS_MT_TOOL_Y",
}

var evRelNames = map[uint16]string{
	0x00: "REL_X", 0x01: "REL_Y", 0x02: "REL_Z",
	0x03: "REL_RX", 0x04: "REL_RY", 0x05: "REL_RZ",
	0x06: "REL_HWHEEL", 0x07: "REL_DIAL",
	0x08: "REL_WHEEL", 0x09: "REL_MISC",
	0x0b: "REL_WHEEL_HI_RES", 0x0c: "REL_HWHEEL_HI_RES",
}
