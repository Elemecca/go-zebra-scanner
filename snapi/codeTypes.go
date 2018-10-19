package snapi

// CodeType provides string IDs for the primary and (optionally)
// secondary symbologies in a symbol.
type CodeType struct {
	primary      string
	supplemental string
}

// CodeTypeTable maps SNAPI codeType ID values to CodeType instances.
var CodeTypeTable = map[uint16]CodeType{
	1:  {"code39", ""},
	2:  {"codabar", ""},
	3:  {"code128", ""},
	4:  {"2of5", ""},
	5:  {"iata", ""},
	6:  {"2of5-int", ""},
	7:  {"code93", ""},
	8:  {"upc-a", ""},
	9:  {"upc-e0", ""},
	10: {"ean-8", ""},
	11: {"ean-13", ""},
	12: {"code11", ""},
	13: {"code49", ""},
	14: {"msi", ""},
	15: {"ean-128", ""},
	16: {"upc-e1", ""},
	17: {"pdf-417", ""},
	18: {"code16k", ""},
	19: {"code39-full", ""},
	20: {"upc-d", ""},
	21: {"code39-tri", ""},
	22: {"bookland", ""},
	23: {"coupon", ""},
	24: {"nw-7", ""},
	25: {"isbt-128", ""},
	26: {"micropdf", ""},
	27: {"datamatrix", ""},
	28: {"qr", ""},
	29: {"micropdf-cca", ""},
	30: {"postnet", ""},
	31: {"planetcode", ""},
	32: {"code32", ""},
	33: {"isbt-128con", ""},
	34: {"postal-jpn", ""},
	35: {"postal-aus", ""},
	36: {"postal-nld", ""},
	37: {"maxicode", ""},
	38: {"postal-can", ""},
	39: {"postal-gbr", ""},
	40: {"macropdf", ""},

	44: {"microqr", ""},
	45: {"aztec", ""},

	48: {"rss-14", ""},
	49: {"rss-limited", ""},
	50: {"rss-expanded", ""},

	55: {"scanlet", ""},

	72: {"upc-a", "sup2"},
	73: {"upc-e0", "sup2"},
	74: {"ean-8", "sup2"},
	75: {"ean-13", "sup2"},

	80: {"upc-e1", "sup2"},
	81: {"ean-128", "cca"},
	82: {"ean-13", "cca"},
	83: {"ean-8", "cca"},
	84: {"rss-expanded", "cca"},
	85: {"rss-limited", "cca"},
	86: {"rss-14", "cca"},
	87: {"upc-a", "cca"},
	88: {"upc-e", "cca"},
	89: {"ean-128", "ccc"},
	90: {"tlc-39", ""},

	97:  {"ean-128", "ccb"},
	98:  {"ean-13", "ccb"},
	99:  {"ean-8", "ccb"},
	100: {"rss-expanded", "ccb"},
	101: {"rss-limited", "ccb"},
	102: {"rss-14", "ccb"},
	103: {"upc-a", "ccb"},
	104: {"upc-e", "ccb"},
	105: {"signature", ""},
	113: {"2of5-matrix", ""},
	114: {"2of5-chn", ""},

	136: {"upc-a", "sup5"},
	137: {"upc-e0", "sup5"},
	138: {"ean-8", "sup5"},
	139: {"ean-13", "sup5"},

	144: {"upc-e1", "sup5"},

	154: {"macro-micropdf", ""},
}

var primaryLengthTable = map[string]int{
	"ean-8":       8,
	"ean-13":      13,
	"rss-14":      16,
	"rss-limited": 16,
	"upc-a":       12,
	"upc-e0":      12,
	// rss-expanded is variable-length
	// ean-128 is variable-length
}
