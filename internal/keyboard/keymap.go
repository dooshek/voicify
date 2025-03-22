package keyboard

// X11 modifier key codes
const (
	X11LeftControl  uint16 = 65507
	X11RightControl uint16 = 65508
	X11LeftShift    uint16 = 65505
	X11RightShift   uint16 = 65506
	X11LeftAlt      uint16 = 65513
	X11RightAlt     uint16 = 65027
	X11Super        uint16 = 65515
)

// Wayland modifier key codes
const (
	WaylandLeftControl  uint16 = 29
	WaylandRightControl uint16 = 97
	WaylandLeftShift    uint16 = 42
	WaylandRightShift   uint16 = 54
	WaylandLeftAlt      uint16 = 56
	WaylandRightAlt     uint16 = 100
	WaylandSuper        uint16 = 125
)

// X11KeyCodes maps key names to their X11 codes
var X11KeyCodes = map[string]uint16{
	// a-z
	"a": 38, "b": 56, "c": 54, "d": 40, "e": 26, "f": 41, "g": 71, "h": 43,
	"i": 31, "j": 44, "k": 45, "l": 46, "m": 58, "n": 57, "o": 32, "p": 33,
	"q": 24, "r": 27, "s": 39, "t": 28, "u": 30, "v": 55, "w": 25, "x": 53,
	"y": 29, "z": 52,
	// 0-9
	"0": 19, "1": 10, "2": 11, "3": 12, "4": 13, "5": 14, "6": 15, "7": 16, "8": 17, "9": 18,
	// Special characters
	"`": 49, "[": 34, "]": 35, "\\": 51, ";": 47, "'": 48, ",": 59, ".": 60, "/": 61, "-": 20, "=": 21,
}

// WaylandKeyCodes maps key names to their Wayland codes
var WaylandKeyCodes = map[string]uint16{
	// a-z
	"a": 30, "b": 48, "c": 46, "d": 32, "e": 18, "f": 33, "g": 34, "h": 35,
	"i": 23, "j": 36, "k": 37, "l": 38, "m": 50, "n": 49, "o": 24, "p": 25,
	"q": 16, "r": 19, "s": 31, "t": 20, "u": 22, "v": 47, "w": 17, "x": 45,
	"y": 21, "z": 44,
	// 0-9
	"0": 11, "1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10,
	// Special characters
	"`": 41, "[": 26, "]": 27, "\\": 43, ";": 39, "'": 40, ",": 51, ".": 52, "/": 53, "-": 12, "=": 13,
}

// X11KeyMap maps X11 codes to key names
var X11KeyMap = map[uint16]string{
	// a-z
	38: "a", 56: "b", 54: "c", 40: "d", 26: "e", 41: "f", 71: "g", 43: "h",
	31: "i", 44: "j", 45: "k", 46: "l", 58: "m", 57: "n", 32: "o", 33: "p",
	24: "q", 27: "r", 39: "s", 28: "t", 30: "u", 55: "v", 25: "w", 53: "x",
	29: "y", 52: "z",
	// Additional ASCII codes for common letters that might be reported differently
	97: "a", 118: "v", 99: "c", 120: "x", 122: "z",
	// 0-9
	19: "0", 10: "1", 11: "2", 12: "3", 13: "4", 14: "5", 15: "6", 16: "7", 17: "8", 18: "9",
	// Special characters
	49: "`", 34: "[", 35: "]", 51: "\\", 47: ";", 48: "'", 59: ",", 60: ".", 61: "/", 20: "-", 21: "=",
	// Additional common X11 codes
	65: "space",
	9:  "escape",
}

// WaylandKeyMap maps Wayland codes to key names
var WaylandKeyMap = map[uint16]string{
	// a-z
	30: "a", 48: "b", 46: "c", 32: "d", 18: "e", 33: "f", 34: "g", 35: "h",
	23: "i", 36: "j", 37: "k", 38: "l", 50: "m", 49: "n", 24: "o", 25: "p",
	16: "q", 19: "r", 31: "s", 20: "t", 22: "u", 47: "v", 17: "w", 45: "x",
	21: "y", 44: "z",
	// 0-9
	11: "0", 2: "1", 3: "2", 4: "3", 5: "4", 6: "5", 7: "6", 8: "7", 9: "8", 10: "9",
	// Special characters
	41: "`", 26: "[", 27: "]", 43: "\\", 39: ";", 40: "'", 51: ",", 52: ".", 53: "/", 12: "-", 13: "=",
	// Additional common Wayland codes
	57: "space",
	1:  "escape",
}
