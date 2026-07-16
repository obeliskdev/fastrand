package fastrand

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unsafe"
)

type RandomizerEncoding int

const (
	RandomizerEncodingNone RandomizerEncoding = 0
	RandomizerEncodingURL  RandomizerEncoding = 1 << iota
	RandomizerEncodingHTML
)

type CustomKeywordGenerator func(length int) []byte

var (
	defaultEngine     *FastEngine
	SafeMailProviders []string
	allKeywords       = []string{
		"ABL", "ABU", "ABR", "DIGIT", "HEX", "SPACE", "UUID",
		"NULL", "IPV4", "IPV6", "BYTES", "EMAIL",
	}
)

//go:embed mail_providers.txt
var mailProviders string

func init() {
	lines := strings.Split(mailProviders, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			SafeMailProviders = append(SafeMailProviders, trimmed)
		}
	}
	defaultEngine = NewEngine()
}

func RandomizerString(payload string) string {
	return defaultEngine.RandomizerString(payload)
}

func Randomizer(payload []byte) []byte {
	return defaultEngine.Randomizer(payload)
}

func (e *FastEngine) RandomizerString(payload string) string {
	if !strings.ContainsAny(payload, "{%&") && e.outputEncoding == RandomizerEncodingNone {
		return payload
	}
	buf := make([]byte, 0, len(payload)+512)
	buf = e.RandomizerAppendString(buf, payload)
	return string(buf)
}

func (e *FastEngine) Randomizer(payload []byte) []byte {
	if !bytes.ContainsAny(payload, "{%&") && e.outputEncoding == RandomizerEncodingNone {
		return payload
	}

	if e.inputEncoding != RandomizerEncodingNone && bytes.ContainsAny(payload, "%&") {
		payload = normalize(payload, e.inputEncoding)
	}

	buf := make([]byte, 0, len(payload)+512)
	e.randomizerInto(payload, &buf)
	return buf
}

func (e *FastEngine) RandomizerAppend(dst []byte, payload []byte) []byte {
	if !bytes.ContainsAny(payload, "{%&") && e.outputEncoding == RandomizerEncodingNone {
		return append(dst, payload...)
	}
	if e.inputEncoding != RandomizerEncodingNone && bytes.ContainsAny(payload, "%&") {
		payload = normalize(payload, e.inputEncoding)
	}
	e.randomizerInto(payload, &dst)
	return dst
}

func (e *FastEngine) RandomizerAppendString(dst []byte, payload string) []byte {
	if !strings.ContainsAny(payload, "{%&") && e.outputEncoding == RandomizerEncodingNone {
		return append(dst, payload...)
	}
	var normalized []byte
	if e.inputEncoding != RandomizerEncodingNone && strings.ContainsAny(payload, "%&") {
		normalized = normalizeString(payload, e.inputEncoding)
	} else {
		normalized = s2b(payload)
	}
	e.randomizerInto(normalized, &dst)
	return dst
}

func (e *FastEngine) randomizerInto(payload []byte, out *[]byte) {
	cursor := 0
	for {
		startIndex := bytes.Index(payload[cursor:], startTag)
		if startIndex == -1 {
			e.writeEncoded(out, payload[cursor:])
			return
		}
		startIndex += cursor
		e.writeEncoded(out, payload[cursor:startIndex])

		cursor = startIndex
		endIndex := bytes.IndexByte(payload[cursor:], endTag)
		if endIndex == -1 {
			e.writeEncoded(out, payload[cursor:])
			return
		}
		endIndex += cursor
		tag := payload[cursor:endIndex]
		cursor = endIndex + 1

		e.parseAndReplaceFast(tag, out)
	}
}

func (e *FastEngine) writeEncoded(out *[]byte, data []byte) {
	if len(data) == 0 {
		return
	}
	switch e.outputEncoding {
	case RandomizerEncodingURL:
		appendURLEncode(out, data)
	case RandomizerEncodingHTML:
		appendHTMLEncode(out, data)
	default:
		*out = append(*out, data...)
	}
}

func appendURLEncode(out *[]byte, data []byte) {
	for _, c := range data {
		if c == ' ' {
			*out = append(*out, '+')
		} else if c < 128 && noEscapeTable[c] {
			*out = append(*out, c)
		} else {
			*out = append(*out, '%')
			*out = append(*out, hexUpper[c>>4], hexUpper[c&0xf])
		}
	}
}

var noEscapeTable = [128]bool{
	'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true, 'G': true,
	'H': true, 'I': true, 'J': true, 'K': true, 'L': true, 'M': true, 'N': true,
	'O': true, 'P': true, 'Q': true, 'R': true, 'S': true, 'T': true, 'U': true,
	'V': true, 'W': true, 'X': true, 'Y': true, 'Z': true,
	'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true,
	'h': true, 'i': true, 'j': true, 'k': true, 'l': true, 'm': true, 'n': true,
	'o': true, 'p': true, 'q': true, 'r': true, 's': true, 't': true, 'u': true,
	'v': true, 'w': true, 'x': true, 'y': true, 'z': true,
	'0': true, '1': true, '2': true, '3': true, '4': true,
	'5': true, '6': true, '7': true, '8': true, '9': true,
	'-': true, '_': true, '.': true, '~': true,
}

func appendHTMLEncode(out *[]byte, data []byte) {
	for _, c := range data {
		switch c {
		case '&':
			*out = append(*out, '&', 'a', 'm', 'p', ';')
		case '\'':
			*out = append(*out, '&', '#', '3', '9', ';')
		case '<':
			*out = append(*out, '&', 'l', 't', ';')
		case '>':
			*out = append(*out, '&', 'g', 't', ';')
		case '"':
			*out = append(*out, '&', '#', '3', '4', ';')
		default:
			*out = append(*out, c)
		}
	}
}

var hexUpper = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F'}

func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

func s2b(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func (e *FastEngine) parseAndReplaceFast(tag []byte, out *[]byte) {
	tag = tag[len(startTag):]
	hasOpt := false
	if bytes.HasPrefix(tag, startTagOpt) {
		tag = tag[len(startTagOpt):]
		hasOpt = true
	}

	if len(tag) == 0 {
		appendString(out, e.defaultLength, e.getCharset(kwABR, CharsAll))
		return
	}

	if tag[0] != sepTag {
		if e.outputEncoding == RandomizerEncodingNone {
			*out = append(*out, startTag...)
			if hasOpt {
				*out = append(*out, startTagOpt...)
			}
			*out = append(*out, tag...)
			*out = append(*out, endTag)
			return
		}
		var tmp [128]byte
		n := copy(tmp[:], startTag)
		if hasOpt {
			n += copy(tmp[n:], startTagOpt)
		}
		n += copy(tmp[n:], tag)
		tmp[n] = endTag
		n++
		e.writeEncoded(out, tmp[:n])
		return
	}
	tag = tag[1:]

	length := e.defaultLength
	var typeKeyword, lenPart []byte

	sepIndex := bytes.IndexByte(tag, sepTag)
	if sepIndex == -1 {
		lenPart = tag
	} else {
		lenPart = tag[:sepIndex]
		typeKeyword = tag[sepIndex+1:]
	}

	var lengthParsed bool
	if e.lengthChoicesEnabled && bytes.IndexByte(lenPart, ',') != -1 {
		var validLengths [16]int
		validCount := 0
		start := 0
		for {
			idx := bytes.IndexByte(lenPart[start:], ',')
			var part []byte
			if idx == -1 {
				part = lenPart[start:]
				if l, ok := parseLengthFast(part); ok && l >= e.minLength && l <= e.maxLength {
					validLengths[validCount] = l
					validCount++
				}
				break
			}
			part = lenPart[start : start+idx]
			if l, ok := parseLengthFast(part); ok && l >= e.minLength && l <= e.maxLength {
				validLengths[validCount] = l
				validCount++
			}
			start += idx + 1
		}

		if validCount > 0 {
			length = validLengths[int(fastUint64N(uint64(validCount)))]
			lengthParsed = true
		}
	}

	if !lengthParsed && e.rangesEnabled && bytes.IndexByte(lenPart, '-') != -1 {
		rangeSepIndex := bytes.IndexByte(lenPart, '-')
		if rangeSepIndex != -1 {
			minPart := lenPart[:rangeSepIndex]
			maxPart := lenPart[rangeSepIndex+1:]
			if minX, ok1 := parseLengthFast(minPart); ok1 && minX >= e.minLength {
				if maxX, ok2 := parseLengthFast(maxPart); ok2 && minX <= maxX && maxX <= e.maxLength {
					length = int(fastUint64N(uint64(maxX-minX+1))) + minX
					lengthParsed = true
				}
			}
		}
	}

	if !lengthParsed {
		if l, ok := parseLengthFast(lenPart); ok && l >= e.minLength && l <= e.maxLength {
			length = l
		} else if typeKeyword == nil {
			typeKeyword = lenPart
		}
	}

	if length < e.minLength {
		length = e.minLength
	}

	if e.keywordChoicesEnabled && bytes.IndexByte(typeKeyword, ',') != -1 {
		var validChoices [16][]byte
		validCount := 0
		start := 0
		for {
			idx := bytes.IndexByte(typeKeyword[start:], ',')
			var choice []byte
			if idx == -1 {
				choice = typeKeyword[start:]
				if e.isKeywordValid(choice) {
					validChoices[validCount] = choice
					validCount++
				}
				break
			}
			choice = typeKeyword[start : start+idx]
			if e.isKeywordValid(choice) {
				validChoices[validCount] = choice
				validCount++
			}
			start += idx + 1
		}
		if validCount > 0 {
			typeKeyword = validChoices[int(fastUint64N(uint64(validCount)))]
		}
	}

	var upperKey string
	if len(e.customKeywords) > 0 || !e.isBuiltinKeywordEnabled(typeKeyword) {
		var key [16]byte
		n := upperASCIIInto(key[:], typeKeyword)
		upperKey = unsafeString(key[:n])
		if customGen, exists := e.customKeywords[upperKey]; exists {
			*out = append(*out, customGen(length)...)
			return
		}
		enabled, exists := e.enabledKeywords[upperKey]
		if !exists || !enabled {
			appendString(out, length, e.getCharset(kwABR, CharsAll))
			return
		}
	} else {
		var key [16]byte
		n := upperASCIIInto(key[:], typeKeyword)
		upperKey = unsafeString(key[:n])
	}

	switch upperKey {
	case "ABL":
		appendString(out, length, e.getCharset(kwABL, CharsAlphabetLower))
	case "ABU":
		appendString(out, length, e.getCharset(kwABU, CharsAlphabetUpper))
	case "ABR":
		appendString(out, length, e.getCharset(kwABR, CharsAlphabet))
	case "DIGIT":
		appendString(out, length, e.getCharset(kwDIGIT, CharsDigits))
	case "NULL":
		nullCharset := e.getCharset(kwNULL, CharsNull)
		nsLen := len(nullCharset)
		if nsLen <= 256 {
			for i := 0; i < length; i++ {
				*out = append(*out, nullCharset[fastUint8N(uint8(nsLen))])
			}
		} else {
			for i := 0; i < length; i++ {
				*out = append(*out, nullCharset[int(fastUint64N(uint64(nsLen)))])
			}
		}
	case "SPACE":
		start := len(*out)
		ensureCap(out, start+length)
		*out = (*out)[:start+length]
		for i := start; i < len(*out); i++ {
			(*out)[i] = ' '
		}
	case "UUID":
		appendUUID(out)
	case "BYTES":
		*out = append(*out, Bytes(length)...)
	case "IPV4":
		appendIPv4(out)
	case "IPV6":
		appendIPv6(out)
	case "EMAIL":
		e.appendRandomEmail(out, length)
	case "HEX":
		appendHex(out, length, e.defaultLength)
	default:
		appendString(out, length, e.getCharset(kwABR, CharsAll))
	}
}

func (e *FastEngine) isBuiltinKeywordEnabled(keyword []byte) bool {
	var key [16]byte
	n := upperASCIIInto(key[:], keyword)
	enabled, exists := e.enabledKeywords[unsafeString(key[:n])]
	return exists && enabled
}

func upperASCIIInto(dst, src []byte) int {
	n := len(src)
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		c := src[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		dst[i] = c
	}
	return n
}

func (e *FastEngine) isKeywordValid(choice []byte) bool {
	var key [16]byte
	n := upperASCIIInto(key[:], choice)
	k := unsafeString(key[:n])
	if _, isCustom := e.customKeywords[k]; isCustom {
		return true
	}
	isEnabled := e.enabledKeywords[k]
	return isEnabled
}

func ensureCap(out *[]byte, n int) {
	if cap(*out) < n {
		bigger := make([]byte, len(*out), n+128)
		copy(bigger, *out)
		*out = bigger
	}
}

func appendString(out *[]byte, length int, charset CharsList) {
	if length <= 0 {
		return
	}
	csLen := len(charset)
	if csLen == 0 {
		return
	}
	start := len(*out)
	ensureCap(out, start+length)
	*out = (*out)[:start+length]
	fillStringInto((*out)[start:], charset, csLen)
}

func appendUUID(out *[]byte) {
	var raw [16]byte
	FillBytes(raw[:])
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	start := len(*out)
	ensureCap(out, start+36)
	*out = (*out)[:start+36]
	b := (*out)[start:]
	hex.Encode(b[0:8], raw[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], raw[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], raw[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], raw[8:10])
	b[23] = '-'
	hex.Encode(b[24:], raw[10:])
}

func appendHex(out *[]byte, byteLength, defaultLen int) {
	if byteLength <= 0 {
		byteLength = defaultLen
	}
	hexLen := byteLength * 2
	start := len(*out)
	ensureCap(out, start+hexLen)
	*out = (*out)[:start+hexLen]
	FillHex((*out)[start:])
}

func appendIPv4(out *[]byte) {
	var raw [4]byte
	FillBytes(raw[:])
	appendUintByte(out, raw[0])
	*out = append(*out, '.')
	appendUintByte(out, raw[1])
	*out = append(*out, '.')
	appendUintByte(out, raw[2])
	*out = append(*out, '.')
	appendUintByte(out, raw[3])
}

func appendUintByte(out *[]byte, v byte) {
	if v < 10 {
		*out = append(*out, '0'+v)
		return
	}
	if v < 100 {
		*out = append(*out, '0'+v/10, '0'+v%10)
		return
	}
	*out = append(*out, '0'+v/100, '0'+(v/10)%10, '0'+v%10)
}

func appendIPv6(out *[]byte) {
	var raw [16]byte
	FillBytes(raw[:])
	for i := 0; i < 8; i++ {
		if i > 0 {
			*out = append(*out, ':')
		}
		val := binary.BigEndian.Uint16(raw[i*2:])
		appendHexUint16(out, val)
	}
}

func appendHexUint16(out *[]byte, val uint16) {
	if val == 0 {
		*out = append(*out, '0')
		return
	}
	var buf [4]byte
	pos := 4
	for val > 0 {
		pos--
		buf[pos] = strconvDigits[val&0xf]
		val >>= 4
	}
	*out = append(*out, buf[pos:]...)
}

func (e *FastEngine) appendRandomEmail(out *[]byte, userLength int) {
	if userLength <= 0 {
		userLength = 8
	}
	provider := "gmail.com"
	if len(e.mailProviders) > 0 {
		provider = e.mailProviders[int(fastUint64N(uint64(len(e.mailProviders))))]
	}
	totalLen := userLength + 1 + len(provider)
	start := len(*out)
	ensureCap(out, start+totalLen)
	*out = (*out)[:start+totalLen]
	b := (*out)[start:]
	fillStringInto(b[:userLength], e.getCharset(kwABL, CharsAlphabetLower), len(CharsAlphabetLower))
	b[userLength] = '@'
	copy(b[userLength+1:], provider)
}

func strconvAppendUint(b []byte, val uint64, base int) []byte {
	var buf [20]byte
	pos := strconvPutUint(buf[:], val, base)
	return append(b, buf[pos:]...)
}

var strconvDigits = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

func strconvPutUint(buf []byte, val uint64, base int) int {
	if val == 0 {
		buf[len(buf)-1] = '0'
		return len(buf) - 1
	}
	pos := len(buf)
	for val > 0 {
		pos--
		buf[pos] = strconvDigits[val%uint64(base)]
		val /= uint64(base)
	}
	return pos
}

func (e *FastEngine) getCharset(keyword []byte, fallback CharsList) CharsList {
	if cs, ok := e.customCharsets[unsafeString(keyword)]; ok {
		return cs
	}
	return fallback
}

var (
	startTag         = []byte("{RAND")
	startUrlEncoded  = []byte("%7BRAND")
	startHtmlEncoded = []byte("&lbrace;RAND")
	startTagOpt      = []byte("OM")
	endTag           = byte('}')
	endTagUrl        = []byte("%7D")
	endTagHtml       = []byte("&rbrace;")
	sepTag           = byte(';')
	sepTagUrl        = []byte("%3B")
	sepTagHtml       = []byte("&semi;")
	kwABL            = []byte("ABL")
	kwABU            = []byte("ABU")
	kwABR            = []byte("ABR")
	kwDIGIT          = []byte("DIGIT")
	kwNULL           = []byte("NULL")
)

type normalizer struct {
	payload       []byte
	encodingFlags RandomizerEncoding
	out           []byte
}

func (n *normalizer) run() []byte {
	cursor := 0
	for cursor < len(n.payload) {
		idx := bytes.IndexAny(n.payload[cursor:], "%&")
		if idx == -1 {
			n.out = append(n.out, n.payload[cursor:]...)
			break
		}
		n.out = append(n.out, n.payload[cursor:cursor+idx]...)
		cursor += idx

		char := n.payload[cursor]

		if char == '%' && (n.encodingFlags&RandomizerEncodingURL != 0) {
			if hasPrefix(n.payload, startUrlEncoded, cursor) {
				n.out = append(n.out, startTag...)
				cursor += len(startUrlEncoded)
			} else if hasPrefix(n.payload, endTagUrl, cursor) {
				n.out = append(n.out, endTag)
				cursor += len(endTagUrl)
			} else if hasPrefix(n.payload, sepTagUrl, cursor) {
				n.out = append(n.out, sepTag)
				cursor += len(sepTagUrl)
			} else {
				n.out = append(n.out, char)
				cursor++
			}
		} else if char == '&' && (n.encodingFlags&RandomizerEncodingHTML != 0) {
			if hasPrefix(n.payload, startHtmlEncoded, cursor) {
				n.out = append(n.out, startTag...)
				cursor += len(startHtmlEncoded)
			} else if hasPrefix(n.payload, endTagHtml, cursor) {
				n.out = append(n.out, endTag)
				cursor += len(endTagHtml)
			} else if hasPrefix(n.payload, sepTagHtml, cursor) {
				n.out = append(n.out, sepTag)
				cursor += len(sepTagHtml)
			} else {
				n.out = append(n.out, char)
				cursor++
			}
		} else {
			n.out = append(n.out, char)
			cursor++
		}
	}
	return n.out
}

func normalize(payload []byte, encodingFlags RandomizerEncoding) []byte {
	if !bytes.ContainsAny(payload, "%&") {
		return payload
	}
	n := normalizer{
		payload:       payload,
		encodingFlags: encodingFlags,
		out:           make([]byte, 0, len(payload)),
	}
	return n.run()
}

func hasPrefix(slice, prefix []byte, pos int) bool {
	if pos+len(prefix) > len(slice) {
		return false
	}
	return bytes.Equal(slice[pos:pos+len(prefix)], prefix)
}

func normalizeString(payload string, encodingFlags RandomizerEncoding) []byte {
	if !strings.ContainsAny(payload, "%&") {
		return []byte(payload)
	}
	n := normalizer{
		payload:       s2b(payload),
		encodingFlags: encodingFlags,
		out:           make([]byte, 0, len(payload)),
	}
	return n.run()
}

func parseLengthFast(b []byte) (int, bool) {
	switch len(b) {
	case 1:
		c := b[0]
		if c >= '0' && c <= '9' {
			return int(c - '0'), true
		}
	case 2:
		c1, c2 := b[0], b[1]
		if c1 >= '0' && c1 <= '9' && c2 >= '0' && c2 <= '9' {
			return int(c1-'0')*10 + int(c2-'0'), true
		}
	case 3:
		c1, c2, c3 := b[0], b[1], b[2]
		if c1 >= '0' && c1 <= '9' && c2 >= '0' && c2 <= '9' && c3 >= '0' && c3 <= '9' {
			return int(c1-'0')*100 + int(c2-'0')*10 + int(c3-'0'), true
		}
	}
	return 0, false
}
