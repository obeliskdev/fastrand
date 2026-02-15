package fastrand

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"html"
	"math/rand"
	"net/url"
	"strings"

	"github.com/valyala/bytebufferpool"
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
	return string(e.Randomizer([]byte(payload)))
}

func (e *FastEngine) Randomizer(payload []byte) []byte {
	if !bytes.ContainsAny(payload, "{%&") && e.outputEncoding == RandomizerEncodingNone {
		return payload
	}

	if e.inputEncoding != RandomizerEncodingNone && bytes.ContainsAny(payload, "%&") {
		payload = normalize(payload, e.inputEncoding)
	}

	buffer := bytebufferpool.Get()
	defer bytebufferpool.Put(buffer)

	cursor := 0
	for {
		startIndex := bytes.Index(payload[cursor:], startTag)
		if startIndex == -1 {
			e.writeEncoded(buffer, payload[cursor:])
			break
		}
		startIndex += cursor
		e.writeEncoded(buffer, payload[cursor:startIndex])

		cursor = startIndex
		endIndex := bytes.IndexByte(payload[cursor:], endTag)
		if endIndex == -1 {
			e.writeEncoded(buffer, payload[cursor:])
			break
		}
		endIndex += cursor
		tag := payload[cursor:endIndex]
		cursor = endIndex + 1

		e.parseAndReplaceFast(tag, buffer)
	}

	result := append([]byte(nil), buffer.Bytes()...)
	return result
}

func (e *FastEngine) writeEncoded(buffer *bytebufferpool.ByteBuffer, data []byte) {
	if len(data) == 0 {
		return
	}
	switch e.outputEncoding {
	case RandomizerEncodingURL:
		_, _ = buffer.WriteString(url.QueryEscape(string(data)))
	case RandomizerEncodingHTML:
		_, _ = buffer.WriteString(html.EscapeString(string(data)))
	default:
		_, _ = buffer.Write(data)
	}
}

func (e *FastEngine) parseAndReplaceFast(tag []byte, buffer *bytebufferpool.ByteBuffer) {
	tag = tag[len(startTag):]
	if bytes.HasPrefix(tag, startTagOpt) {
		tag = tag[len(startTagOpt):]
	}

	if len(tag) == 0 {
		_, _ = buffer.WriteString(String(e.defaultLength, CharsAll))
		return
	}

	if tag[0] != sepTag {
		tempBuf := bytebufferpool.Get()
		defer bytebufferpool.Put(tempBuf)
		_, _ = tempBuf.Write(startTag)
		if bytes.HasPrefix(tag, startTagOpt) {
			_, _ = tempBuf.Write(startTagOpt)
		}
		_, _ = tempBuf.Write(tag)
		e.writeEncoded(buffer, tempBuf.Bytes())
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
		var validLengths []int
		start := 0
		for {
			idx := bytes.IndexByte(lenPart[start:], ',')
			var part []byte
			if idx == -1 {
				part = lenPart[start:]
				if l, ok := parseLengthFast(part); ok && l >= e.minLength && l <= e.maxLength {
					validLengths = append(validLengths, l)
				}
				break
			}
			part = lenPart[start : start+idx]
			if l, ok := parseLengthFast(part); ok && l >= e.minLength && l <= e.maxLength {
				validLengths = append(validLengths, l)
			}
			start += idx + 1
		}

		if len(validLengths) > 0 {
			length = validLengths[rand.Intn(len(validLengths))]
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
					length = rand.Intn(maxX-minX+1) + minX
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
		var validChoices [][]byte
		start := 0
		for {
			idx := bytes.IndexByte(typeKeyword[start:], ',')
			var choice []byte
			if idx == -1 {
				choice = typeKeyword[start:]
				upcasedChoice := strings.ToUpper(string(choice))
				_, isCustom := e.customKeywords[upcasedChoice]
				isEnabled := e.enabledKeywords[upcasedChoice]
				if isCustom || isEnabled {
					validChoices = append(validChoices, choice)
				}
				break
			}
			choice = typeKeyword[start : start+idx]
			upcasedChoice := strings.ToUpper(string(choice))
			_, isCustom := e.customKeywords[upcasedChoice]
			isEnabled := e.enabledKeywords[upcasedChoice]
			if isCustom || isEnabled {
				validChoices = append(validChoices, choice)
			}
			start += idx + 1
		}
		if len(validChoices) > 0 {
			typeKeyword = validChoices[rand.Intn(len(validChoices))]
		}
	}

	upcasedKeyword := strings.ToUpper(string(typeKeyword))
	if customGen, exists := e.customKeywords[upcasedKeyword]; exists {
		_, _ = buffer.Write(customGen(length))
		return
	}

	if enabled, exists := e.enabledKeywords[upcasedKeyword]; !exists || !enabled {
		_, _ = buffer.WriteString(String(length, e.getCharset(kwABR, CharsAll)))
		return
	}

	switch {
	case bytes.EqualFold(typeKeyword, kwABL):
		_, _ = buffer.WriteString(String(length, e.getCharset(kwABL, CharsAlphabetLower)))
	case bytes.EqualFold(typeKeyword, kwABU):
		_, _ = buffer.WriteString(String(length, e.getCharset(kwABU, CharsAlphabetUpper)))
	case bytes.EqualFold(typeKeyword, kwABR):
		_, _ = buffer.WriteString(String(length, e.getCharset(kwABR, CharsAlphabet)))
	case bytes.EqualFold(typeKeyword, kwDIGIT):
		_, _ = buffer.WriteString(String(length, e.getCharset(kwDIGIT, CharsDigits)))
	case bytes.EqualFold(typeKeyword, kwNULL):
		nullCharset := e.getCharset(kwNULL, CharsNull)
		for i := 0; i < length; i++ {
			_ = buffer.WriteByte(Choice(nullCharset))
		}
	case bytes.EqualFold(typeKeyword, kwSPACE):
		for i := 0; i < length; i++ {
			_ = buffer.WriteByte(' ')
		}
	case bytes.EqualFold(typeKeyword, kwUUID):
		_, _ = buffer.Write(generateUUID())
	case bytes.EqualFold(typeKeyword, kwBYTES):
		_, _ = buffer.Write(Bytes(length))
	case bytes.EqualFold(typeKeyword, kwIPV4):
		_, _ = buffer.WriteString(IPv4().String())
	case bytes.EqualFold(typeKeyword, kwIPV6):
		_, _ = buffer.WriteString(IPv6().String())
	case bytes.EqualFold(typeKeyword, kwEMAIL):
		_, _ = buffer.Write(e.generateRandomEmail(length))
	case bytes.EqualFold(typeKeyword, kwHEX):
		_, _ = buffer.Write(generateRandomHex(length, e.defaultLength))
	default:
		_, _ = buffer.WriteString(String(length, e.getCharset(kwABR, CharsAll)))
	}
}

func (e *FastEngine) getCharset(keyword []byte, fallback CharsList) CharsList {
	if cs, ok := e.customCharsets[string(keyword)]; ok {
		return cs
	}
	return fallback
}

func (e *FastEngine) generateRandomEmail(userLength int) []byte {
	if userLength <= 0 {
		userLength = 8
	}
	user := String(userLength, e.getCharset(kwABL, CharsAlphabetLower))
	provider := "gmail.com"
	if len(e.mailProviders) > 0 {
		provider = Choice(e.mailProviders)
	}

	emailLen := len(user) + 1 + len(provider)
	b := make([]byte, emailLen)
	copy(b, user)
	b[len(user)] = '@'
	copy(b[len(user)+1:], provider)
	return b
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
	normalizedBuf := bytebufferpool.Get()
	defer bytebufferpool.Put(normalizedBuf)

	cursor := 0
	for cursor < len(payload) {
		idx := bytes.IndexAny(payload[cursor:], "%&")
		if idx == -1 {
			_, _ = normalizedBuf.Write(payload[cursor:])
			break
		}
		_, _ = normalizedBuf.Write(payload[cursor : cursor+idx])
		cursor += idx

		char := payload[cursor]

		if char == '%' && (encodingFlags&RandomizerEncodingURL != 0) {
			if hasPrefix(payload, startUrlEncoded, cursor) {
				_, _ = normalizedBuf.Write(startTag)
				cursor += len(startUrlEncoded)
			} else if hasPrefix(payload, endTagUrl, cursor) {
				_ = normalizedBuf.WriteByte(endTag)
				cursor += len(endTagUrl)
			} else if hasPrefix(payload, sepTagUrl, cursor) {
				_ = normalizedBuf.WriteByte(sepTag)
				cursor += len(sepTagUrl)
			} else {
				_ = normalizedBuf.WriteByte(payload[cursor])
				cursor++
			}
		} else if char == '&' && (encodingFlags&RandomizerEncodingHTML != 0) {
			if hasPrefix(payload, startHtmlEncoded, cursor) {
				_, _ = normalizedBuf.Write(startTag)
				cursor += len(startHtmlEncoded)
			} else if hasPrefix(payload, endTagHtml, cursor) {
				_ = normalizedBuf.WriteByte(endTag)
				cursor += len(endTagHtml)
			} else if hasPrefix(payload, sepTagHtml, cursor) {
				_ = normalizedBuf.WriteByte(sepTag)
				cursor += len(sepTagHtml)
			} else {
				_ = normalizedBuf.WriteByte(payload[cursor])
				cursor++
			}
		} else {
			_ = normalizedBuf.WriteByte(payload[cursor])
			cursor++
		}
	}
	result := append([]byte(nil), normalizedBuf.Bytes()...)
	return result
}

func hasPrefix(slice, prefix []byte, pos int) bool {
	if pos+len(prefix) > len(slice) {
		return false
	}
	return bytes.Equal(slice[pos:pos+len(prefix)], prefix)
}

func generateUUID() []byte {
	uuid, _ := FastUUID()
	b := make([]byte, 36)
	hex.Encode(b[0:8], uuid[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], uuid[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], uuid[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], uuid[8:10])
	b[23] = '-'
	hex.Encode(b[24:], uuid[10:])
	return b
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

func generateRandomHex(byteLength, defaultLen int) []byte {
	if byteLength <= 0 {
		byteLength = defaultLen
	}
	srcBytes := Bytes(byteLength)
	hexBytes := make([]byte, byteLength*2)
	hex.Encode(hexBytes, srcBytes)
	return hexBytes
}
