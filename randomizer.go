package fastrand

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"html"
	"net/url"
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
	if e.inputEncoding != RandomizerEncodingNone && strings.ContainsAny(payload, "%&") {
		normalized := normalizeString(payload, e.inputEncoding)
		e.randomizerInto(normalized, &buf)
	} else {
		e.randomizerInto([]byte(payload), &buf)
	}
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
		*out = append(*out, url.QueryEscape(unsafeString(data))...)
	case RandomizerEncodingHTML:
		*out = append(*out, html.EscapeString(unsafeString(data))...)
	default:
		*out = append(*out, data...)
	}
}

func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
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
		tmp := make([]byte, 0, len(startTag)+boolToInt(hasOpt)*2+len(tag)+1)
		tmp = append(tmp, startTag...)
		if hasOpt {
			tmp = append(tmp, startTagOpt...)
		}
		tmp = append(tmp, tag...)
		tmp = append(tmp, endTag)
		e.writeEncoded(out, tmp)
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
	if e.lengthChoicesEnabled && bytes.Contains(lenPart, []byte(",")) {
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

	if !lengthParsed && e.rangesEnabled && bytes.Contains(lenPart, []byte("-")) {
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

	if e.keywordChoicesEnabled && bytes.Contains(typeKeyword, []byte(",")) {
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

	if len(e.customKeywords) > 0 {
		var key [16]byte
		n := upperASCIIInto(key[:], typeKeyword)
		if customGen, exists := e.customKeywords[string(key[:n])]; exists {
			*out = append(*out, customGen(length)...)
			return
		}
	}

	if !e.isBuiltinKeywordEnabled(typeKeyword) {
		appendString(out, length, e.getCharset(kwABR, CharsAll))
		return
	}

	switch {
	case bytesEqualFold(typeKeyword, kwABL):
		appendString(out, length, e.getCharset(kwABL, CharsAlphabetLower))
	case bytesEqualFold(typeKeyword, kwABU):
		appendString(out, length, e.getCharset(kwABU, CharsAlphabetUpper))
	case bytesEqualFold(typeKeyword, kwABR):
		appendString(out, length, e.getCharset(kwABR, CharsAlphabet))
	case bytesEqualFold(typeKeyword, kwDIGIT):
		appendString(out, length, e.getCharset(kwDIGIT, CharsDigits))
	case bytesEqualFold(typeKeyword, kwNULL):
		nullCharset := e.getCharset(kwNULL, CharsNull)
		for i := 0; i < length; i++ {
			*out = append(*out, nullCharset[int(fastUint64N(uint64(len(nullCharset))))])
		}
	case bytesEqualFold(typeKeyword, kwSPACE):
		for i := 0; i < length; i++ {
			*out = append(*out, ' ')
		}
	case bytesEqualFold(typeKeyword, kwUUID):
		appendUUID(out)
	case bytesEqualFold(typeKeyword, kwBYTES):
		*out = append(*out, Bytes(length)...)
	case bytesEqualFold(typeKeyword, kwIPV4):
		appendIPv4(out)
	case bytesEqualFold(typeKeyword, kwIPV6):
		appendIPv6(out)
	case bytesEqualFold(typeKeyword, kwEMAIL):
		e.appendRandomEmail(out, length)
	case bytesEqualFold(typeKeyword, kwHEX):
		appendHex(out, length, e.defaultLength)
	default:
		appendString(out, length, e.getCharset(kwABR, CharsAll))
	}
}

func (e *FastEngine) isBuiltinKeywordEnabled(keyword []byte) bool {
	var key [16]byte
	n := upperASCIIInto(key[:], keyword)
	enabled, exists := e.enabledKeywords[string(key[:n])]
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

func bytesEqualFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 32
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func (e *FastEngine) isKeywordValid(choice []byte) bool {
	var key [16]byte
	n := upperASCIIInto(key[:], choice)
	k := string(key[:n])
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
	*out = strconvAppendUint(*out, uint64(raw[0]), 10)
	*out = append(*out, '.')
	*out = strconvAppendUint(*out, uint64(raw[1]), 10)
	*out = append(*out, '.')
	*out = strconvAppendUint(*out, uint64(raw[2]), 10)
	*out = append(*out, '.')
	*out = strconvAppendUint(*out, uint64(raw[3]), 10)
}

func appendIPv6(out *[]byte) {
	var raw [16]byte
	FillBytes(raw[:])
	for i := 0; i < 8; i++ {
		if i > 0 {
			*out = append(*out, ':')
		}
		val := binary.BigEndian.Uint16(raw[i*2:])
		*out = strconvAppendUint(*out, uint64(val), 16)
	}
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

func strconvPutUint(buf []byte, val uint64, base int) int {
	if val == 0 {
		buf[len(buf)-1] = '0'
		return len(buf) - 1
	}
	const digits = "0123456789abcdef"
	pos := len(buf)
	for val > 0 {
		pos--
		buf[pos] = digits[val%uint64(base)]
		val /= uint64(base)
	}
	return pos
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
	kwHEX            = []byte("HEX")
	kwSPACE          = []byte("SPACE")
	kwUUID           = []byte("UUID")
	kwNULL           = []byte("NULL")
	kwIPV4           = []byte("IPV4")
	kwIPV6           = []byte("IPV6")
	kwBYTES          = []byte("BYTES")
	kwEMAIL          = []byte("EMAIL")
)

func normalize(payload []byte, encodingFlags RandomizerEncoding) []byte {
	var buf []byte
	if cap(buf) < len(payload) {
		buf = make([]byte, 0, len(payload))
	}

	cursor := 0
	for cursor < len(payload) {
		idx := bytes.IndexAny(payload[cursor:], "%&")
		if idx == -1 {
			buf = append(buf, payload[cursor:]...)
			break
		}
		buf = append(buf, payload[cursor:cursor+idx]...)
		cursor += idx

		char := payload[cursor]

		if char == '%' && (encodingFlags&RandomizerEncodingURL != 0) {
			if hasPrefix(payload, startUrlEncoded, cursor) {
				buf = append(buf, startTag...)
				cursor += len(startUrlEncoded)
			} else if hasPrefix(payload, endTagUrl, cursor) {
				buf = append(buf, endTag)
				cursor += len(endTagUrl)
			} else if hasPrefix(payload, sepTagUrl, cursor) {
				buf = append(buf, sepTag)
				cursor += len(sepTagUrl)
			} else {
				buf = append(buf, payload[cursor])
				cursor++
			}
		} else if char == '&' && (encodingFlags&RandomizerEncodingHTML != 0) {
			if hasPrefix(payload, startHtmlEncoded, cursor) {
				buf = append(buf, startTag...)
				cursor += len(startHtmlEncoded)
			} else if hasPrefix(payload, endTagHtml, cursor) {
				buf = append(buf, endTag)
				cursor += len(endTagHtml)
			} else if hasPrefix(payload, sepTagHtml, cursor) {
				buf = append(buf, sepTag)
				cursor += len(sepTagHtml)
			} else {
				buf = append(buf, payload[cursor])
				cursor++
			}
		} else {
			buf = append(buf, payload[cursor])
			cursor++
		}
	}
	return buf
}

func hasPrefix(slice, prefix []byte, pos int) bool {
	if pos+len(prefix) > len(slice) {
		return false
	}
	return bytes.Equal(slice[pos:pos+len(prefix)], prefix)
}

func hasPrefixString(s string, prefix string, pos int) bool {
	if pos+len(prefix) > len(s) {
		return false
	}
	return s[pos:pos+len(prefix)] == prefix
}

func normalizeString(payload string, encodingFlags RandomizerEncoding) []byte {
	buf := make([]byte, 0, len(payload))
	cursor := 0
	for cursor < len(payload) {
		idx := strings.IndexAny(payload[cursor:], "%&")
		if idx == -1 {
			buf = append(buf, payload[cursor:]...)
			break
		}
		buf = append(buf, payload[cursor:cursor+idx]...)
		cursor += idx

		char := payload[cursor]

		if char == '%' && (encodingFlags&RandomizerEncodingURL != 0) {
			if hasPrefixString(payload, string(startUrlEncoded), cursor) {
				buf = append(buf, startTag...)
				cursor += len(startUrlEncoded)
			} else if hasPrefixString(payload, string(endTagUrl), cursor) {
				buf = append(buf, endTag)
				cursor += len(endTagUrl)
			} else if hasPrefixString(payload, string(sepTagUrl), cursor) {
				buf = append(buf, sepTag)
				cursor += len(sepTagUrl)
			} else {
				buf = append(buf, payload[cursor])
				cursor++
			}
		} else if char == '&' && (encodingFlags&RandomizerEncodingHTML != 0) {
			if hasPrefixString(payload, string(startHtmlEncoded), cursor) {
				buf = append(buf, startTag...)
				cursor += len(startHtmlEncoded)
			} else if hasPrefixString(payload, string(endTagHtml), cursor) {
				buf = append(buf, endTag)
				cursor += len(endTagHtml)
			} else if hasPrefixString(payload, string(sepTagHtml), cursor) {
				buf = append(buf, sepTag)
				cursor += len(sepTagHtml)
			} else {
				buf = append(buf, payload[cursor])
				cursor++
			}
		} else {
			buf = append(buf, payload[cursor])
			cursor++
		}
	}
	return buf
}

func parseLengthFast(b []byte) (int, bool) {
	if len(b) == 1 {
		c := b[0]
		if c >= '0' && c <= '9' {
			return int(c - '0'), true
		}
	}
	if len(b) == 2 {
		c1, c2 := b[0], b[1]
		if c1 >= '0' && c1 <= '9' && c2 >= '0' && c2 <= '9' {
			return int(c1-'0')*10 + int(c2-'0'), true
		}
	}
	return 0, false
}
