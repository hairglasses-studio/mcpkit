package device

import "testing"

func TestClassifyDevice_AllVendorBrands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		vendorID  uint16
		productID uint16
		devName   string
		want      DeviceType
	}{
		// Vendor heuristic: game controllers
		{"nintendo vendor", 0x057e, 0xFFFF, "Unknown", TypeGamepad},
		{"valve vendor", 0x28de, 0x0001, "Steam Controller", TypeGamepad},
		{"8bitdo vendor", 0x2dc8, 0x0001, "8BitDo Pro 2", TypeGamepad},

		// Vendor heuristic: HOTAS
		{"vkb vendor", 0x231d, 0xFFFF, "VKB Unknown", TypeHOTAS},
		{"virpil vendor", 0x3344, 0xFFFF, "Virpil Unknown", TypeHOTAS},

		// Vendor heuristic: racing wheels
		{"fanatec vendor", 0x0eb7, 0xFFFF, "Fanatec Unknown", TypeRacingWheel},
		{"thrustmaster wheel vendor", 0x044f, 0xFFFF, "Thrustmaster Unknown", TypeHOTAS}, // 044f maps to thrustmaster_flight

		// Vendor heuristic: MIDI controllers
		{"akai vendor", 0x09e8, 0xFFFF, "Akai Unknown", TypeMIDI},
		{"korg vendor", 0x0944, 0xFFFF, "KORG Unknown", TypeMIDI},
		{"roland vendor", 0x0582, 0xFFFF, "Roland Unknown", TypeMIDI},
		{"casio vendor", 0x07cf, 0xFFFF, "Casio Unknown", TypeMIDI},
		{"yamaha vendor", 0x0499, 0xFFFF, "Yamaha Unknown", TypeMIDI},
		{"native_instruments vendor", 0x17cc, 0xFFFF, "NI Unknown", TypeMIDI},
		{"behringer vendor", 0x1397, 0xFFFF, "Behringer Unknown", TypeMIDI},
		{"novation vendor", 0x1410, 0xFFFF, "Novation Unknown", TypeMIDI},
		{"m_audio vendor", 0x0763, 0xFFFF, "M-Audio Unknown", TypeMIDI},
		{"keith_mcmillen vendor", 0x2011, 0xFFFF, "KMI Unknown", TypeMIDI},
		{"roli vendor", 0x314b, 0xFFFF, "ROLI Unknown", TypeMIDI},
		{"focusrite vendor", 0x1235, 0xFFFF, "Focusrite Unknown", TypeMIDI},
		{"arturia vendor", 0x1c75, 0xFFFF, "Arturia Unknown", TypeMIDI},

		// Vendor heuristic: Stream Deck
		{"elgato vendor unknown PID", 0x0fd9, 0xFFFF, "Elgato Unknown", TypeGenericHID},

		// Vendors that do NOT have vendor-level heuristic (brand not in switch)
		{"pdp vendor no heuristic", 0x0e6f, 0xFFFF, "PDP Controller", TypeGamepad},     // name fallback
		{"powera vendor no heuristic", 0x24c6, 0xFFFF, "PowerA Controller", TypeGamepad}, // name fallback
		{"logitech vendor no heuristic", 0x046d, 0xFFFF, "Logitech Unknown", TypeUnknown},
		{"razer vendor no heuristic", 0x1532, 0xFFFF, "Razer Unknown", TypeUnknown},
		{"madcatz vendor no heuristic", 0x0738, 0xFFFF, "Mad Catz Unknown", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyDevice(tt.vendorID, tt.productID, tt.devName)
			if got != tt.want {
				t.Errorf("ClassifyDevice(0x%04x, 0x%04x, %q) = %v, want %v",
					tt.vendorID, tt.productID, tt.devName, got, tt.want)
			}
		})
	}
}

func TestClassifyDevice_NameHeuristics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		devName string
		want    DeviceType
	}{
		// MIDI name patterns (checked before gamepad because "controller" overlap)
		{"midi keyword", "USB MIDI Interface", TypeMIDI},
		{"launchpad", "Novation Launchpad Mini", TypeMIDI},
		{"apc", "Akai APC40", TypeMIDI},
		{"nanocontrol", "KORG nanocontrol Studio", TypeMIDI},
		{"beatstep", "Arturia BeatStep Pro", TypeMIDI},
		{"mpk", "Akai MPK Mini mk3", TypeMIDI},
		{"keystation", "M-Audio Keystation 49", TypeMIDI},
		{"grid name", "Intech Grid Module", TypeMIDI},

		// Gamepad name patterns
		{"controller", "Generic USB Controller", TypeGamepad},
		{"gamepad", "My Custom Gamepad", TypeGamepad},
		{"joystick", "Retro Joystick", TypeGamepad},
		{"xbox name", "xbox compatible pad", TypeGamepad},
		{"dualshock name", "dualshock compatible", TypeGamepad},
		{"dualsense name", "dualsense wired", TypeGamepad},
		{"pro controller", "Switch Pro Controller Clone", TypeGamepad},

		// Generic HID name patterns
		{"stream deck", "Elgato Stream Deck Mini", TypeGenericHID},
		{"pedal", "USB Foot Pedal", TypeGenericHID},
		{"macropad", "Custom Macropad", TypeGenericHID},
		{"keypad", "Numeric Keypad", TypeGenericHID},

		// HOTAS name patterns
		{"hotas", "VKB HOTAS Setup", TypeHOTAS},
		{"flight", "USB Flight Stick", TypeHOTAS},
		{"throttle", "CH Throttle Quadrant", TypeHOTAS},
		{"rudder", "Saitek Pro Flight Rudder", TypeHOTAS},
		{"flight stick", "Generic Flight Stick", TypeHOTAS},

		// Racing wheel name patterns
		{"wheel", "Logitech Racing Wheel", TypeRacingWheel},
		{"pedals", "Fanatec CSL Pedals LC", TypeGenericHID}, // "pedal" matches generic_hid before "pedals" matches racing_wheel
		{"shifter", "Thrustmaster TH8A Shifter", TypeRacingWheel},
		{"handbrake", "Aiologs USB Handbrake", TypeRacingWheel},

		// Keyboard
		{"keyboard", "QMK Keyboard with Encoders", TypeKeyboard},

		// Mouse
		{"mouse", "Logitech MX Master Mouse", TypeMouse},
		{"trackball", "Kensington Trackball", TypeMouse},

		// Unknown (no matching pattern)
		{"unknown device", "Mystery Gadget 3000", TypeUnknown},
		{"empty name", "", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Use zero vendor/product to force name-based fallback.
			got := ClassifyDevice(0, 0, tt.devName)
			if got != tt.want {
				t.Errorf("ClassifyDevice(0, 0, %q) = %v, want %v",
					tt.devName, got, tt.want)
			}
		})
	}
}

func TestClassifyDevice_KnownProductIDs(t *testing.T) {
	t.Parallel()

	// Spot-check a few from each category to ensure the knownDevices map
	// takes priority over vendor heuristics.
	tests := []struct {
		name      string
		vendorID  uint16
		productID uint16
		want      DeviceType
	}{
		{"Xbox One S", 0x045e, 0x02ea, TypeGamepad},
		{"DualSense Edge", 0x054c, 0x0df2, TypeGamepad},
		{"Switch Pro", 0x057e, 0x2009, TypeGamepad},
		{"APC Key 25", 0x09e8, 0x003a, TypeMIDI},
		{"Launchpad X", 0x1235, 0x0069, TypeMIDI},
		{"Maschine mk3", 0x17cc, 0x1620, TypeMIDI},
		{"nanoKONTROL2", 0x0944, 0x0117, TypeMIDI},
		{"T.16000M FCS", 0x044f, 0xb10a, TypeHOTAS},
		{"Gladiator NXT EVO", 0x231d, 0x0126, TypeHOTAS},
		{"G29", 0x046d, 0xc24f, TypeRacingWheel},
		{"CSL DD", 0x0eb7, 0x0004, TypeRacingWheel},
		{"T300 RS", 0x044f, 0xb66d, TypeRacingWheel},
		{"Stream Deck MK.2", 0x0fd9, 0x0080, TypeGenericHID},
		{"Stream Deck Pedal", 0x0fd9, 0x0084, TypeGenericHID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyDevice(tt.vendorID, tt.productID, "irrelevant name")
			if got != tt.want {
				t.Errorf("ClassifyDevice(0x%04x, 0x%04x) = %v, want %v",
					tt.vendorID, tt.productID, got, tt.want)
			}
		})
	}
}

func TestClassifyDevice_CaseInsensitive(t *testing.T) {
	t.Parallel()

	// Name matching should be case-insensitive.
	tests := []struct {
		name string
		want DeviceType
	}{
		{"USB MIDI Controller", TypeMIDI},
		{"usb midi controller", TypeMIDI},
		{"USB Midi Controller", TypeMIDI},
		{"GAMEPAD PRO", TypeGamepad},
		{"Keyboard Layout", TypeKeyboard},
	}
	for _, tt := range tests {
		got := ClassifyDevice(0, 0, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyDevice(0, 0, %q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestClassifyDevice_MIDIBeforeGamepad(t *testing.T) {
	// "MIDI Controller" contains "controller" but should match MIDI first.
	got := ClassifyDevice(0, 0, "MIDI Controller")
	if got != TypeMIDI {
		t.Errorf("'MIDI Controller' classified as %v, want midi (MIDI should be checked before gamepad)", got)
	}
}

func TestBrandLabel_AllVendors(t *testing.T) {
	t.Parallel()

	// Every vendor in VendorBrands should return its brand.
	for vid, brand := range VendorBrands {
		got := BrandLabel(vid)
		if got != brand {
			t.Errorf("BrandLabel(0x%04x) = %q, want %q", vid, got, brand)
		}
	}
}

func TestBrandLabel_UnknownVendor(t *testing.T) {
	if got := BrandLabel(0x0000); got != "unknown" {
		t.Errorf("BrandLabel(0x0000) = %q, want unknown", got)
	}
}

func TestContainsAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s      string
		subs   []string
		expect bool
	}{
		{"hello world", []string{"hello"}, true},
		{"hello world", []string{"xyz", "world"}, true},
		{"hello world", []string{"xyz", "abc"}, false},
		{"", []string{"anything"}, false},
		{"notempty", []string{}, false},
		{"midi controller", []string{"midi"}, true},
	}

	for _, tt := range tests {
		got := containsAny(tt.s, tt.subs...)
		if got != tt.expect {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.subs, got, tt.expect)
		}
	}
}
