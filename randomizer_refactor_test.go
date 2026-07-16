package fastrand_test

import (
	"bytes"
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/obeliskdev/fastrand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomizerAppend(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		engine := fastrand.NewEngine()
		result := engine.RandomizerAppend(nil, []byte("{RAND;10;DIGIT}"))
		assert.Len(t, result, 10, "RandomizerAppend should produce correct length")
		checkCharset(t, result, fastrand.CharsDigits)
	})

	t.Run("PreAllocatedBuffer", func(t *testing.T) {
		engine := fastrand.NewEngine()
		dst := make([]byte, 0, 512)
		result := engine.RandomizerAppend(dst, []byte("{RAND;8;ABL}"))
		assert.Len(t, result, 8)
		checkCharset(t, result, fastrand.CharsAlphabetLower)
	})

	t.Run("PreservesExistingContent", func(t *testing.T) {
		engine := fastrand.NewEngine()
		dst := make([]byte, 0, 512)
		dst = append(dst, "prefix-"...)
		result := engine.RandomizerAppend(dst, []byte("{RAND;4;DIGIT}"))
		assert.True(t, strings.HasPrefix(string(result), "prefix-"), "Existing content should be preserved")
		assert.Len(t, result, len("prefix-")+4)
		checkCharset(t, result[len("prefix-"):], fastrand.CharsDigits)
	})

	t.Run("MultipleAppends", func(t *testing.T) {
		engine := fastrand.NewEngine()
		dst := make([]byte, 0, 512)
		dst = engine.RandomizerAppend(dst, []byte("{RAND;4;DIGIT}"))
		assert.Len(t, dst, 4)
		dst = engine.RandomizerAppend(dst, []byte(" "))
		assert.Len(t, dst, 5)
		dst = engine.RandomizerAppend(dst, []byte("{RAND;4;DIGIT}"))
		assert.Len(t, dst, 9)
	})

	t.Run("NoPlaceholders", func(t *testing.T) {
		engine := fastrand.NewEngine()
		result := engine.RandomizerAppend(nil, []byte("hello world"))
		assert.Equal(t, "hello world", string(result))
	})

	t.Run("EmptyPayload", func(t *testing.T) {
		engine := fastrand.NewEngine()
		result := engine.RandomizerAppend(nil, []byte{})
		assert.Empty(t, result)
	})

	t.Run("EqualsRandomizer", func(t *testing.T) {
		engine := fastrand.NewEngine()
		payload := []byte("User:{RAND;10;ABL}|Sess:{RAND;32;HEX}|ID:{RAND;UUID}")
		r1 := engine.Randomizer(payload)
		r2 := engine.RandomizerAppend(nil, payload)
		assert.Len(t, r1, len(r2), "Randomizer and RandomizerAppend should produce same length")
	})

	t.Run("CapacityGrowth", func(t *testing.T) {
		engine := fastrand.NewEngine()
		dst := make([]byte, 0, 8)
		result := engine.RandomizerAppend(dst, []byte("{RAND;50;ABL}"))
		assert.Len(t, result, 50, "Should grow when capacity is insufficient")
		checkCharset(t, result, fastrand.CharsAlphabetLower)
	})

	t.Run("WithOutputEncoding", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingURL))
		result := engine.RandomizerAppend(nil, []byte("foo=bar&baz={RAND;4;HEX}"))
		expectedPrefix := "foo%3Dbar%26baz%3D"
		assert.True(t, strings.HasPrefix(string(result), expectedPrefix), "Output should be URL encoded")
	})

	t.Run("WithCustomKeyword", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomKeyword("SKU", func(length int) []byte {
				return []byte("SKU-" + fastrand.String(length, fastrand.CharsDigits))
			}),
		)
		result := engine.RandomizerAppend(nil, []byte("ID-{RAND;8;SKU}-End"))
		assert.True(t, strings.HasPrefix(string(result), "ID-SKU-"), "Custom keyword should work with Append")
		assert.True(t, strings.HasSuffix(string(result), "-End"), "Suffix should be preserved")
	})
}

func TestRandomizerAppendConcurrency(t *testing.T) {
	t.Parallel()
	engine := fastrand.NewEngine()
	payload := []byte("User:{RAND;10;ABL}|Sess:{RAND;32;HEX}|ID:{RAND;UUID}")

	const numGoroutines = 50
	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				result := engine.RandomizerAppend(nil, payload)
				if len(result) == 0 {
					t.Errorf("RandomizerAppend returned empty result")
				}
			}
		}()
	}
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestRandomizerKeywordCaseInsensitive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		keyword string
		check   func(testing.TB, []byte)
		minLen  int
	}{
		{"abl_lower", "{RAND;10;abl}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsAlphabetLower) }, 10},
		{"ABU_upper", "{RAND;10;ABU}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsAlphabetUpper) }, 10},
		{"AbR_mixed", "{RAND;10;AbR}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsAlphabet) }, 10},
		{"dIgIt_mixed", "{RAND;10;dIgIt}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsDigits) }, 10},
		{"hex_lower", "{RAND;8;hex}", checkHexFormat, 16},
		{"Hex_mixed", "{RAND;8;Hex}", checkHexFormat, 16},
		{"UUID_upper", "{RAND;UUID}", checkUUIDFormat, 36},
		{"uuid_lower", "{RAND;uuid}", checkUUIDFormat, 36},
		{"UuId_mixed", "{RAND;UuId}", checkUUIDFormat, 36},
		{"ipv4_lower", "{RAND;ipv4}", checkIPv4Format, -1},
		{"IPV4_upper", "{RAND;IPV4}", checkIPv4Format, -1},
		{"ipv6_lower", "{RAND;ipv6}", checkIPv6Format, -1},
		{"IPV6_upper", "{RAND;IPV6}", checkIPv6Format, -1},
		{"SPACE_upper", "{RAND;5;SPACE}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsSpace) }, 5},
		{"space_lower", "{RAND;5;space}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsSpace) }, 5},
		{"NULL_upper", "{RAND;7;NULL}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsNull) }, 7},
		{"null_lower", "{RAND;7;null}", func(tb testing.TB, b []byte) { checkCharset(tb, b, fastrand.CharsNull) }, 7},
		{"BYTES_upper", "{RAND;10;BYTES}", nil, 10},
		{"bytes_lower", "{RAND;10;bytes}", nil, 10},
		{"EMAIL_upper", "{RAND;8;EMAIL}", checkEmailFormat, -1},
		{"email_lower", "{RAND;8;email}", checkEmailFormat, -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fastrand.RandomizerString(tc.keyword)
			if tc.minLen > 0 {
				assert.Len(t, result, tc.minLen, "Keyword %s: expected length %d, got %d", tc.keyword, tc.minLen, len(result))
			}
			if tc.check != nil {
				tc.check(t, []byte(result))
			}
		})
	}
}

func TestRandomizerKeywordChoiceCaseInsensitive(t *testing.T) {
	t.Parallel()

	t.Run("MixedCaseChoices", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result := fastrand.RandomizerString("{RAND;uuid,hex}")
			isUUID := uuidRegex.MatchString(result)
			isHex := hexRegex.MatchString(result)
			assert.True(t, isUUID || isHex, "Mixed-case choice should produce UUID or HEX, got %q", result)
		}
	})

	t.Run("MixedCaseChoicesUpper", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result := fastrand.RandomizerString("{RAND;UUID,HEX}")
			isUUID := uuidRegex.MatchString(result)
			isHex := hexRegex.MatchString(result)
			assert.True(t, isUUID || isHex, "Upper-case choice should produce UUID or HEX, got %q", result)
		}
	})

	t.Run("MixedCaseChoicesMixed", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result := fastrand.RandomizerString("{RAND;UuId,HeX}")
			isUUID := uuidRegex.MatchString(result)
			isHex := hexRegex.MatchString(result)
			assert.True(t, isUUID || isHex, "Mixed-case choice should produce UUID or HEX, got %q", result)
		}
	})
}

func TestRandomizerDisabledKeywordCaseInsensitive(t *testing.T) {
	t.Parallel()

	t.Run("DisableLower", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithDisabledKeywords("uuid"))
		result := engine.RandomizerString("{RAND;UUID}")
		assert.False(t, uuidRegex.MatchString(result), "Disabled keyword (lower) should not produce UUID")
		assert.Len(t, result, 16, "Disabled keyword should fall back to default length")
	})

	t.Run("DisableUpper", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithDisabledKeywords("UUID"))
		result := engine.RandomizerString("{RAND;uuid}")
		assert.False(t, uuidRegex.MatchString(result), "Disabled keyword (upper) should not produce UUID")
		assert.Len(t, result, 16, "Disabled keyword should fall back to default length")
	})

	t.Run("DisableMixed", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithDisabledKeywords("UuId"))
		result := engine.RandomizerString("{RAND;uuid}")
		assert.False(t, uuidRegex.MatchString(result), "Disabled keyword (mixed) should not produce UUID")
		assert.Len(t, result, 16, "Disabled keyword should fall back to default length")
	})
}

func TestRandomizerCustomKeywordCaseInsensitive(t *testing.T) {
	t.Parallel()

	t.Run("CustomLower", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomKeyword("sku", func(length int) []byte {
				return []byte("SKU-" + fastrand.String(length, fastrand.CharsDigits))
			}),
		)
		result := engine.RandomizerString("{RAND;8;SKU}")
		assert.True(t, strings.HasPrefix(string(result), "SKU-"), "Custom keyword (registered lower) should work")
	})

	t.Run("CustomUpper", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomKeyword("SKU", func(length int) []byte {
				return []byte("SKU-" + fastrand.String(length, fastrand.CharsDigits))
			}),
		)
		result := engine.RandomizerString("{RAND;8;sku}")
		assert.True(t, strings.HasPrefix(string(result), "SKU-"), "Custom keyword (registered upper) should work with lower input")
	})

	t.Run("CustomMixed", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomKeyword("SkU", func(length int) []byte {
				return []byte("SKU-" + fastrand.String(length, fastrand.CharsDigits))
			}),
		)
		result := engine.RandomizerString("{RAND;8;sku}")
		assert.True(t, strings.HasPrefix(string(result), "SKU-"), "Custom keyword (registered mixed) should work with lower input")
	})
}

func TestRandomizerCustomCharsetCaseInsensitive(t *testing.T) {
	t.Parallel()

	t.Run("CustomCharsetLower", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomCharset("digit", []byte("01")),
		)
		result := engine.RandomizerString("{RAND;10;DIGIT}")
		checkCharset(t, []byte(result), []byte("01"))
	})

	t.Run("CustomCharsetUpper", func(t *testing.T) {
		engine := fastrand.NewEngine(
			fastrand.WithCustomCharset("DIGIT", []byte("01")),
		)
		result := engine.RandomizerString("{RAND;10;digit}")
		checkCharset(t, []byte(result), []byte("01"))
	})
}

func TestRandomizerMalformedTags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"MalformedOptionalTag", "Value: {RANDOMfoo}", "Value: {RANDOMfoo}"},
		{"MalformedOptionalTagNoOM", "Value: {RANDfoo}", "Value: {RANDfoo}"},
		{"IncompleteStart", "Value: {RAN", "Value: {RAN"},
		{"IncompleteMiddle", "Value: {RAND;10", "Value: {RAND;10"},
		{"IncompleteWithType", "Value: {RAND;10;HEX", "Value: {RAND;10;HEX"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fastrand.RandomizerString(tc.input)
			assert.Equal(t, tc.expected, result, "Malformed tag should round-trip")
		})
	}
}

func TestRandomizerIPv4Format(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("IP: {RAND;IPV4}")
		ipStr := strings.TrimPrefix(result, "IP: ")
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip, "Generated IPv4 should be valid: %q", ipStr)
		require.NotNil(t, ip.To4(), "Generated IP should be IPv4: %q", ipStr)
		assert.False(t, strings.Contains(ipStr, ":"), "IPv4 should not contain colons")
	}
}

func TestRandomizerIPv6Format(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("IP: {RAND;IPV6}")
		ipStr := strings.TrimPrefix(result, "IP: ")
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip, "Generated IPv6 should be valid: %q", ipStr)
		assert.Nil(t, ip.To4(), "Generated IP should not be IPv4-mapped: %q", ipStr)
		assert.True(t, strings.Contains(ipStr, ":"), "IPv6 should contain colons")
	}
}

func TestRandomizerEmailFormat(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("Contact: {RAND;8;EMAIL}")
		emailStr := strings.TrimPrefix(result, "Contact: ")
		checkEmailFormat(t, []byte(emailStr))
	}
}

func TestRandomizerUUIDFormat(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("UUID: {RAND;UUID}")
		uuidStr := strings.TrimPrefix(result, "UUID: ")
		assert.True(t, uuidRegex.MatchString(uuidStr), "Generated UUID should match V4 format: %q", uuidStr)

		// Check version bit (char at index 14 should be '4')
		assert.Equal(t, byte('4'), uuidStr[14], "UUID version char should be '4': %q", uuidStr)
		// Check variant bit (char at index 19 should be 8, 9, a, or b)
		variantChar := uuidStr[19]
		assert.True(t, variantChar == '8' || variantChar == '9' || variantChar == 'a' || variantChar == 'b',
			"UUID variant char should be 8/9/a/b, got %c: %q", variantChar, uuidStr)
	}
}

func TestRandomizerHexFormat(t *testing.T) {
	t.Parallel()
	lengths := []int{1, 2, 4, 8, 16, 32, 64, 99}
	for _, length := range lengths {
		t.Run("Len"+itoa(length), func(t *testing.T) {
			result := fastrand.RandomizerString("{RAND;" + itoa(length) + ";HEX}")
			expectedHexLen := length * 2
			assert.Len(t, result, expectedHexLen, "HEX length %d should produce %d hex chars", length, expectedHexLen)
			assert.True(t, isValidHexStr(result), "HEX output should be valid hex: %q", result)
		})
	}
}

func TestRandomizerSpaceKeyword(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		result := fastrand.RandomizerString("A{RAND;10;SPACE}B")
		assert.Len(t, result, 12, "SPACE keyword should produce exact length")
		for j, c := range result {
			if j == 0 || j == 11 {
				continue
			}
			assert.Equal(t, rune(' '), c, "SPACE keyword should produce spaces at index %d", j)
		}
	}
}

func TestRandomizerNullKeyword(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		result := fastrand.RandomizerString("{RAND;7;NULL}")
		assert.Len(t, result, 7)
		for j, b := range []byte(result) {
			assert.True(t, b >= 0 && b <= 15, "NULL keyword byte at index %d should be 0-15, got %d", j, b)
		}
	}
}

func TestRandomizerBytesKeyword(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		result := fastrand.RandomizerString("Raw: {RAND;10;BYTES}")
		assert.Len(t, result, len("Raw: ")+10, "BYTES keyword should produce exact length")
	}
}

func TestRandomizerLengthRange(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("{RAND;5-10;DIGIT}")
		assert.GreaterOrEqual(t, len(result), 5, "Length range should produce >= min")
		assert.LessOrEqual(t, len(result), 10, "Length range should produce <= max")
		checkCharset(t, []byte(result), fastrand.CharsDigits)
	}
}

func TestRandomizerLengthChoices(t *testing.T) {
	t.Parallel()
	lengths := make(map[int]int)
	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("{RAND;5,10,15;DIGIT}")
		lengths[len(result)]++
		checkCharset(t, []byte(result), fastrand.CharsDigits)
	}
	assert.GreaterOrEqual(t, len(lengths), 2, "Length choices should produce at least 2 different lengths")
	for l := range lengths {
		assert.Contains(t, []int{5, 10, 15}, l, "Length should be one of the choices")
	}
}

func TestRandomizerMinMaxLengthEnforcement(t *testing.T) {
	t.Parallel()

	t.Run("BelowMin", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithMinLength(10), fastrand.WithMaxLength(50), fastrand.WithDefaultLength(10))
		result := engine.RandomizerString("{RAND;2;DIGIT}")
		assert.Len(t, result, 10, "Length below min should fall back to default length")
	})

	t.Run("AboveMax", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithMinLength(1), fastrand.WithMaxLength(20), fastrand.WithDefaultLength(20))
		result := engine.RandomizerString("{RAND;30;DIGIT}")
		assert.Len(t, result, 20, "Length above max should fall back to default length")
	})

	t.Run("WithinRange", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithMinLength(1), fastrand.WithMaxLength(20))
		result := engine.RandomizerString("{RAND;15;DIGIT}")
		assert.Len(t, result, 15, "Length within range should be used as-is")
	})
}

func TestRandomizerDisabledFeatures(t *testing.T) {
	t.Parallel()

	t.Run("DisabledRanges", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithRanges(false))
		result := engine.RandomizerString("{RAND;5-10;DIGIT}")
		assert.Len(t, result, 16, "Disabled ranges should produce default length")
	})

	t.Run("DisabledKeywordChoices", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithKeywordChoices(false))
		result := engine.RandomizerString("{RAND;HEX,UUID}")
		assert.Len(t, result, 16, "Disabled keyword choices should produce default length")
	})

	t.Run("DisabledLengthChoices", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithLengthChoices(false))
		result := engine.RandomizerString("{RAND;5,10,15;DIGIT}")
		assert.Len(t, result, 16, "Disabled length choices should produce default length")
	})
}

func TestRandomizerInputEncoding(t *testing.T) {
	t.Parallel()

	t.Run("URLEncoded", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL))
		result := engine.RandomizerString("%7BRAND;4;HEX%7D")
		assert.Len(t, result, 8, "URL-encoded tag should be processed")
		assert.True(t, hexRegex.MatchString(result), "URL-encoded HEX should be valid hex")
	})

	t.Run("HTMLEncoded", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithInputEncoding(fastrand.RandomizerEncodingHTML))
		result := engine.RandomizerString("&lbrace;RAND;4;HEX&rbrace;")
		assert.Len(t, result, 8, "HTML-encoded tag should be processed")
		assert.True(t, hexRegex.MatchString(result), "HTML-encoded HEX should be valid hex")
	})

	t.Run("BothEncodings", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL | fastrand.RandomizerEncodingHTML))
		resultURL := engine.RandomizerString("%7BRAND;4;HEX%7D")
		assert.Len(t, resultURL, 8, "URL-encoded tag should be processed with both encodings")
		resultHTML := engine.RandomizerString("&lbrace;RAND;4;HEX&rbrace;")
		assert.Len(t, resultHTML, 8, "HTML-encoded tag should be processed with both encodings")
	})

	t.Run("NoneEncoding", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithInputEncoding(fastrand.RandomizerEncodingNone))
		result := engine.RandomizerString("%7BRAND;4;HEX%7D")
		assert.NotEqual(t, 8, len(result), "None encoding should not process URL-encoded tags")
	})
}

func TestRandomizerOutputEncoding(t *testing.T) {
	t.Parallel()

	t.Run("URLEncoding", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingURL))
		result := engine.RandomizerString("foo=bar&baz={RAND;4;HEX}")
		expectedPrefix := "foo%3Dbar%26baz%3D"
		assert.True(t, strings.HasPrefix(string(result), expectedPrefix), "Output should be URL encoded")
		hexPart := strings.TrimPrefix(string(result), expectedPrefix)
		assert.Len(t, hexPart, 8, "Random part should not be encoded")
	})

	t.Run("HTMLEncoding", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingHTML))
		result := engine.RandomizerString("<tag>{RAND;4;HEX}</tag>")
		assert.True(t, strings.HasPrefix(string(result), "&lt;tag&gt;"), "Output should be HTML encoded")
		assert.True(t, strings.HasSuffix(string(result), "&lt;/tag&gt;"), "Output should be HTML encoded")
	})

	t.Run("NoneEncoding", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingNone))
		result := engine.RandomizerString("foo=bar&baz={RAND;4;HEX}")
		assert.True(t, strings.HasPrefix(string(result), "foo=bar&baz="), "Output should not be encoded")
	})
}

func TestRandomizerStringVsBytes(t *testing.T) {
	t.Parallel()

	payloads := []string{
		"User: {RAND;10;ABL} | Session: {RANDOM;32;HEX} | ID: {RAND;UUID}",
		"{RAND;5;DIGIT}{RAND;4;ABL}",
		"IP: {RAND;IPV4} | IP6: {RAND;IPV6}",
		"Email: {RAND;8;EMAIL}",
		"Data: {RAND;5-10;ABU}",
		"No placeholders here",
		"",
	}

	for _, payload := range payloads {
		t.Run("Payload_"+payload, func(t *testing.T) {
			r1 := fastrand.RandomizerString(payload)
			r2 := string(fastrand.Randomizer([]byte(payload)))
			if payload == "" {
				assert.Empty(t, r1)
				assert.Empty(t, r2)
			} else if !strings.Contains(payload, "{RAND") {
				assert.Equal(t, payload, r1, "No-placeholder should pass through")
				assert.Equal(t, payload, r2, "No-placeholder should pass through")
			} else {
				assert.NotEmpty(t, r1, "RandomizerString should produce non-empty result")
				assert.NotEmpty(t, r2, "Randomizer should produce non-empty result")
			}
		})
	}
}

func TestRandomizerReset(t *testing.T) {
	t.Parallel()

	engine := fastrand.NewEngine(
		fastrand.WithDefaultLength(5),
		fastrand.WithDisabledKeywords("UUID"),
	)
	assert.Len(t, engine.RandomizerString("{RAND}"), 5, "Pre-condition: default length is 5")

	engine.Reset()
	assert.Len(t, engine.RandomizerString("{RAND}"), 16, "After Reset: default length should be 16")
	result := engine.RandomizerString("{RAND;UUID}")
	assert.True(t, uuidRegex.MatchString(result), "After Reset: UUID should be re-enabled")
}

func TestRandomizerLargePayload(t *testing.T) {
	t.Parallel()

	var parts []string
	for i := 0; i < 100; i++ {
		parts = append(parts, "{RAND;8;DIGIT}")
	}
	payload := strings.Join(parts, "-")
	result := fastrand.RandomizerString(payload)
	assert.Len(t, result, 100*8+99, "Large payload with 100 placeholders should produce correct length")
}

func TestRandomizerAdjacentPlaceholders(t *testing.T) {
	t.Parallel()

	t.Run("TwoDigits", func(t *testing.T) {
		result := fastrand.RandomizerString("{RAND;3;DIGIT}{RAND;4;ABL}")
		assert.Len(t, result, 7)
		checkCharset(t, []byte(result[:3]), fastrand.CharsDigits)
		checkCharset(t, []byte(result[3:]), fastrand.CharsAlphabetLower)
	})

	t.Run("ThreeMixed", func(t *testing.T) {
		result := fastrand.RandomizerString("{RAND;2;DIGIT}{RAND;2;ABU}{RAND;2;ABL}")
		assert.Len(t, result, 6)
		checkCharset(t, []byte(result[:2]), fastrand.CharsDigits)
		checkCharset(t, []byte(result[2:4]), fastrand.CharsAlphabetUpper)
		checkCharset(t, []byte(result[4:]), fastrand.CharsAlphabetLower)
	})

	t.Run("UUIDAdjacent", func(t *testing.T) {
		result := fastrand.RandomizerString("{RAND;UUID}{RAND;UUID}")
		assert.Len(t, result, 72, "Two adjacent UUIDs should be 72 chars")
		parts := strings.SplitN(result, "-", 2)
		assert.Len(t, parts, 2, "Should contain UUID separator")
	})
}

func TestRandomizerNoPlaceholderPassthrough(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"hello world",
		"no placeholders at all",
		"just some text with { but no RAND",
		"RAND without braces",
		"{} empty braces",
		"{RANDX} close but not RAND",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			result := fastrand.RandomizerString(tc)
			assert.Equal(t, tc, result, "No-placeholder input should pass through unchanged")
		})
	}
}

func TestRandomizerStringNoAlloc(t *testing.T) {
	t.Parallel()

	t.Run("NoPlaceholderNoAlloc", func(t *testing.T) {
		payload := "just some text without placeholders"
		result := fastrand.RandomizerString(payload)
		assert.Equal(t, payload, result, "No-placeholder string should return input directly")
	})
}

func TestRandomizerKeywordChoiceWithDisabled(t *testing.T) {
	t.Parallel()

	t.Run("SomeDisabled", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithDisabledKeywords("UUID"))
		for i := 0; i < 100; i++ {
			result := engine.RandomizerString("{RAND;UUID,HEX}")
			assert.False(t, uuidRegex.MatchString(result), "Disabled UUID should not be generated")
			assert.True(t, hexRegex.MatchString(result), "Only enabled HEX should be generated: %q", result)
		}
	})

	t.Run("AllDisabled", func(t *testing.T) {
		engine := fastrand.NewEngine(fastrand.WithDisabledKeywords("UUID", "HEX"))
		for i := 0; i < 100; i++ {
			result := engine.RandomizerString("{RAND;UUID,HEX}")
			assert.Len(t, result, 16, "All disabled should fall back to default")
		}
	})
}

func TestRandomizerCombinedRangeAndChoice(t *testing.T) {
	t.Parallel()

	for i := 0; i < 1000; i++ {
		result := fastrand.RandomizerString("{RAND;8-12;HEX,ABL}")
		isHex := hexRegex.MatchString(result)
		isABL := true
		for _, char := range []byte(result) {
			if !bytes.Contains([]byte(fastrand.CharsAlphabetLower), []byte{char}) {
				isABL = false
				break
			}
		}
		assert.True(t, isHex || isABL, "Should produce HEX or ABL: %q", result)
		if isHex {
			assert.GreaterOrEqual(t, len(result), 16, "HEX should be >= 16 (8*2)")
			assert.LessOrEqual(t, len(result), 24, "HEX should be <= 24 (12*2)")
			assert.Equal(t, 0, len(result)%2, "HEX length should be even")
		} else {
			assert.GreaterOrEqual(t, len(result), 8, "ABL should be >= 8")
			assert.LessOrEqual(t, len(result), 12, "ABL should be <= 12")
		}
	}
}

func TestRandomizerCombinedRangeChoiceAndDisabled(t *testing.T) {
	t.Parallel()

	engine := fastrand.NewEngine(
		fastrand.WithDisabledKeywords("HEX"),
	)
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;8-12;HEX,ABL}")
		isHex := hexRegex.MatchString(result)
		isABL := true
		for _, char := range []byte(result) {
			if !bytes.Contains([]byte(fastrand.CharsAlphabetLower), []byte{char}) {
				isABL = false
				break
			}
		}
		assert.False(t, isHex, "Disabled HEX should not be generated")
		assert.True(t, isABL, "Only ABL should be generated: %q", result)
		assert.GreaterOrEqual(t, len(result), 8)
		assert.LessOrEqual(t, len(result), 12)
	}
}

func TestRandomizerEmptyInput(t *testing.T) {
	t.Parallel()

	t.Run("EmptyString", func(t *testing.T) {
		result := fastrand.RandomizerString("")
		assert.Empty(t, result)
	})

	t.Run("EmptyBytes", func(t *testing.T) {
		result := fastrand.Randomizer([]byte{})
		assert.Empty(t, result)
	})

	t.Run("EmptyAppend", func(t *testing.T) {
		engine := fastrand.NewEngine()
		result := engine.RandomizerAppend(nil, []byte{})
		assert.Empty(t, result)
	})
}

func TestRandomizerKitchenSink(t *testing.T) {
	t.Parallel()

	engine := fastrand.NewEngine(
		fastrand.WithMinLength(4),
		fastrand.WithMaxLength(80),
		fastrand.WithOutputEncoding(fastrand.RandomizerEncodingHTML),
		fastrand.WithDisabledKeywords("IPV6"),
		fastrand.WithCustomKeyword("USER", func(length int) []byte {
			return []byte("user_" + fastrand.String(length, fastrand.CharsAlphabetLower))
		}),
	)
	template := "<user id='{RAND;10-15;USER}'>Activity: {RAND;IPV4,IPV6}</user>"
	result := engine.RandomizerString(template)

	assert.True(t, strings.HasPrefix(result, "&lt;user id=&#39;user_"), "Output encoding and custom keyword should work")

	startMarker := "Activity: "
	endMarker := "&lt;/user&gt;"
	startIndex := strings.Index(result, startMarker)
	require.NotEqual(t, -1, startIndex, "Should find start marker")
	startIndex += len(startMarker)
	endIndex := strings.Index(result, endMarker)
	require.NotEqual(t, -1, endIndex, "Should find end marker")

	generatedIP := result[startIndex:endIndex]
	assert.False(t, strings.Contains(generatedIP, ":"), "Disabled IPV6 should not be generated")
	ip := net.ParseIP(generatedIP)
	require.NotNil(t, ip, "Generated IP should be valid: %q", generatedIP)
	require.NotNil(t, ip.To4(), "Generated IP should be IPv4: %q", generatedIP)
}

var _ = regexp.MustCompile

func isValidHexStr(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
