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
		}
	}

	// 3. Name-based heuristics (fallback).
	// MIDI checked before gamepad because "MIDI Controller" contains "controller".
	lower := strings.ToLower(name)
	if containsAny(lower, "midi", "launchpad", "apc", "nanocontrol", "beatstep", "mpk", "keystation") {
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
	// Elgato Stream Deck
	{0x0fd9, 0x0060}: TypeGenericHID, // Stream Deck Original
	{0x0fd9, 0x006d}: TypeGenericHID, // Stream Deck Mini
	{0x0fd9, 0x0063}: TypeGenericHID, // Stream Deck XL
	{0x0fd9, 0x0080}: TypeGenericHID, // Stream Deck MK.2
	{0x0fd9, 0x0084}: TypeGenericHID, // Stream Deck Pedal
	{0x0fd9, 0x0086}: TypeGenericHID, // Stream Deck +
	{0x0fd9, 0x008f}: TypeGenericHID, // Stream Deck Neo
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
