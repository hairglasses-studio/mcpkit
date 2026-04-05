package device

import "strings"

// ClassifyDevice determines the DeviceType from USB descriptor information
// and device name heuristics.
func ClassifyDevice(vendorID, productID uint16, name string) DeviceType {
	// 1. Known vendor/product database.
	if dt, ok := knownDevices[deviceKey{vendorID, productID}]; ok {
		return dt
	}

	// 2. Vendor heuristics.
	if brand, ok := VendorBrands[vendorID]; ok {
		switch brand {
		case "xbox", "playstation", "nintendo", "valve", "8bitdo":
			return TypeGamepad
		case "elgato":
			return TypeGenericHID
		case "thrustmaster_flight", "vkb", "virpil":
			return TypeHOTAS
		case "fanatec", "thrustmaster_wheel":
			return TypeRacingWheel
		case "akai", "focusrite", "novation", "arturia", "korg", "roland",
			"casio", "yamaha", "native_instruments", "behringer",
			"m_audio", "keith_mcmillen", "roli":
			return TypeMIDI
		}
	}

	// 3. Name-based heuristics (fallback).
	// MIDI checked before gamepad because "MIDI Controller" contains "controller".
	lower := strings.ToLower(name)
	if containsAny(lower, "midi", "launchpad", "apc", "nanocontrol", "beatstep", "mpk", "keystation", "intech", "grid") {
		return TypeMIDI
	}
	if containsAny(lower, "controller", "gamepad", "joystick", "xbox", "dualshock", "dualsense", "pro controller") {
		return TypeGamepad
	}
	if containsAny(lower, "stream deck", "pedal", "macropad", "keypad") {
		return TypeGenericHID
	}
	if containsAny(lower, "hotas", "flight", "throttle", "rudder", "flight stick") {
		return TypeHOTAS
	}
	if containsAny(lower, "wheel", "pedals", "shifter", "handbrake") {
		return TypeRacingWheel
	}
	if containsAny(lower, "keyboard") {
		return TypeKeyboard
	}
	if containsAny(lower, "mouse", "trackball") {
		return TypeMouse
	}

	return TypeUnknown
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

type deviceKey struct {
	vendorID  uint16
	productID uint16
}

// knownDevices maps specific vendor+product IDs to device types.
var knownDevices = map[deviceKey]DeviceType{
	// --- Xbox Controllers (Microsoft 045e) ---
	{0x045e, 0x02dd}: TypeGamepad, // Xbox One Controller (2015)
	{0x045e, 0x02ea}: TypeGamepad, // Xbox One S Controller
	{0x045e, 0x0b12}: TypeGamepad, // Xbox Wireless Controller (USB)
	{0x045e, 0x0b13}: TypeGamepad, // Xbox Series X|S Controller
	{0x045e, 0x0b00}: TypeGamepad, // Xbox Elite Wireless Controller Series 2
	{0x045e, 0x0b22}: TypeGamepad, // Xbox Elite Wireless Controller Series 2 (USB-C)
	{0x045e, 0x0b0a}: TypeGamepad, // Xbox Adaptive Controller

	// --- PlayStation Controllers (Sony 054c) ---
	{0x054c, 0x05c4}: TypeGamepad, // DualShock 4 (1st gen, CUH-ZCT1)
	{0x054c, 0x09cc}: TypeGamepad, // DualShock 4 (2nd gen, CUH-ZCT2)
	{0x054c, 0x0ce6}: TypeGamepad, // DualSense (PS5)
	{0x054c, 0x0df2}: TypeGamepad, // DualSense Edge (PS5)

	// --- Nintendo Controllers (057e) ---
	{0x057e, 0x2009}: TypeGamepad, // Switch Pro Controller
	{0x057e, 0x2006}: TypeGamepad, // Joy-Con L
	{0x057e, 0x2007}: TypeGamepad, // Joy-Con R

	// --- MIDI Controllers (Akai 09e8) ---
	{0x09e8, 0x003a}: TypeMIDI, // APC Key 25
	{0x09e8, 0x0040}: TypeMIDI, // APC Mini mk2
	{0x09e8, 0x0044}: TypeMIDI, // MPK Mini mk3
	{0x09e8, 0x004f}: TypeMIDI, // APC40 mk2
	{0x09e8, 0x0028}: TypeMIDI, // MPD218
	{0x09e8, 0x0030}: TypeMIDI, // MPK Mini Play

	// --- MIDI Controllers (Novation 1235) ---
	{0x1235, 0x0020}: TypeMIDI, // Launchpad S
	{0x1235, 0x0051}: TypeMIDI, // Launchpad Mini mk3
	{0x1235, 0x0069}: TypeMIDI, // Launchpad X
	{0x1235, 0x006b}: TypeMIDI, // Launchpad Pro mk3
	{0x1235, 0x0113}: TypeMIDI, // Launchkey Mini mk3
	{0x1235, 0x0102}: TypeMIDI, // Launch Control XL

	// --- MIDI Controllers (Arturia 1c75) ---
	{0x1c75, 0x0206}: TypeMIDI, // BeatStep
	{0x1c75, 0x0208}: TypeMIDI, // BeatStep Pro
	{0x1c75, 0x0288}: TypeMIDI, // KeyStep
	{0x1c75, 0x028a}: TypeMIDI, // MiniLab mkII
	{0x1c75, 0x028b}: TypeMIDI, // KeyLab Essential 49

	// --- MIDI Controllers (Native Instruments 17cc) ---
	{0x17cc, 0x1620}: TypeMIDI, // Maschine mk3
	{0x17cc, 0x1600}: TypeMIDI, // Maschine Jam
	{0x17cc, 0x1140}: TypeMIDI, // Komplete Kontrol S49 mk2
	{0x17cc, 0x0815}: TypeMIDI, // Traktor Kontrol S2 mk3

	// --- MIDI Controllers (Korg 0944) ---
	{0x0944, 0x0117}: TypeMIDI, // nanoKONTROL2
	{0x0944, 0x0118}: TypeMIDI, // nanoPAD2
	{0x0944, 0x0113}: TypeMIDI, // nanoKEY2

	// --- MIDI Controllers (Roland 0582) ---
	{0x0582, 0x01d6}: TypeMIDI, // DJ-505
	{0x0582, 0x01db}: TypeMIDI, // DJ-808

	// --- HOTAS (Thrustmaster 044f) ---
	{0x044f, 0xb10a}: TypeHOTAS, // T.16000M FCS
	{0x044f, 0xb687}: TypeHOTAS, // TWCS Throttle
	{0x044f, 0x0402}: TypeHOTAS, // Warthog Joystick
	{0x044f, 0x0404}: TypeHOTAS, // Warthog Throttle

	// --- HOTAS (VKB 231d) ---
	{0x231d, 0x0126}: TypeHOTAS, // Gladiator NXT EVO
	{0x231d, 0x0127}: TypeHOTAS, // Gladiator NXT EVO (left)

	// --- HOTAS (Virpil 3344) ---
	{0x3344, 0x0194}: TypeHOTAS, // VPC Constellation Alpha
	{0x3344, 0x8194}: TypeHOTAS, // VPC MongoosT-50CM3 Throttle

	// --- Racing Wheels (Logitech 046d) ---
	{0x046d, 0xc24f}: TypeRacingWheel, // G29 Driving Force
	{0x046d, 0xc262}: TypeRacingWheel, // G920 Driving Force
	{0x046d, 0xc266}: TypeRacingWheel, // G923 (PlayStation)
	{0x046d, 0xc267}: TypeRacingWheel, // G923 (Xbox)

	// --- Racing Wheels (Fanatec 0eb7) ---
	{0x0eb7, 0x0001}: TypeRacingWheel, // CSL Elite Wheel Base
	{0x0eb7, 0x0004}: TypeRacingWheel, // CSL DD
	{0x0eb7, 0x0005}: TypeRacingWheel, // DD1 / DD2
	{0x0eb7, 0x0011}: TypeRacingWheel, // Podium DD

	// --- Racing Wheels (Thrustmaster 044f) ---
	{0x044f, 0xb66d}: TypeRacingWheel, // T300 RS
	{0x044f, 0xb66e}: TypeRacingWheel, // T300 RS GT Edition
	{0x044f, 0xb66f}: TypeRacingWheel, // T-GT
	{0x044f, 0xb65d}: TypeRacingWheel, // T150
	{0x044f, 0xb669}: TypeRacingWheel, // T248

	// --- Intech Studio Grid (03eb for D51/SAMD51, 303a for ESP32-S3) ---
	{0x03eb, 0xecad}: TypeMIDI, // Grid (Gen1 D51) — EN16, EF44, PO16, BU16, PBF4, TEK2
	{0x03eb, 0xecac}: TypeMIDI, // Grid (Gen1 D51 legacy)
	{0x303a, 0x8123}: TypeMIDI, // Grid (Gen2 ESP32-S3) — current production

	// --- Elgato Stream Deck (0fd9) ---
	{0x0fd9, 0x0060}: TypeGenericHID, // Stream Deck Original
	{0x0fd9, 0x006d}: TypeGenericHID, // Stream Deck Mini
	{0x0fd9, 0x0063}: TypeGenericHID, // Stream Deck XL
	{0x0fd9, 0x0080}: TypeGenericHID, // Stream Deck MK.2
	{0x0fd9, 0x0084}: TypeGenericHID, // Stream Deck Pedal
	{0x0fd9, 0x0086}: TypeGenericHID, // Stream Deck +
	{0x0fd9, 0x008f}: TypeGenericHID, // Stream Deck Neo
	{0x0fd9, 0x008e}: TypeGenericHID, // Stream Deck Mini (v2)
	{0x0fd9, 0x0090}: TypeGenericHID, // Stream Deck XL (v2)
}

// VendorBrands maps USB vendor IDs to brand identifiers.
var VendorBrands = map[uint16]string{
	// Game controllers
	0x045e: "xbox",        // Microsoft
	0x054c: "playstation", // Sony
	0x057e: "nintendo",    // Nintendo
	0x28de: "valve",       // Valve (Steam Controller, Steam Deck)
	0x2dc8: "8bitdo",      // 8BitDo
	0x0e6f: "pdp",         // PDP (Performance Designed Products)
	0x24c6: "powera",      // PowerA
	0x0738: "madcatz",     // Mad Catz
	0x1532: "razer",       // Razer
	0x046d: "logitech",    // Logitech

	// MIDI controllers
	0x1235: "focusrite",   // Focusrite / Novation
	0x09e8: "akai",        // Akai Professional
	0x1c75: "arturia",     // Arturia
	0x0944: "korg",        // KORG
	0x0582: "roland",      // Roland
	0x07cf: "casio",       // Casio
	0x0499: "yamaha",      // Yamaha
	0x17cc: "native_instruments", // Native Instruments
	0x1397: "behringer",   // Behringer / Music Tribe
	0x1410: "novation",    // Novation
	0x0763: "m_audio",     // M-Audio
	0x2011: "keith_mcmillen", // Keith McMillen Instruments
	0x314b: "roli",        // ROLI

	// Stream Deck / macro pads
	0x0fd9: "elgato", // Elgato

	// Flight sim
	0x044f: "thrustmaster_flight", // Thrustmaster
	0x231d: "vkb",                 // VKB
	0x3344: "virpil",              // Virpil

	// Racing
	0x0eb7: "fanatec",             // Fanatec
	// 0x044f also Thrustmaster for racing wheels, handled by product ID
}

// BrandLabel returns a human-friendly label for a vendor brand.
func BrandLabel(vendorID uint16) string {
	if brand, ok := VendorBrands[vendorID]; ok {
		return brand
	}
	return "unknown"
}
