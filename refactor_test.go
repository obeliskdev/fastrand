package fastrand_test

import (
	"strings"
	"testing"

	"github.com/obeliskdev/fastrand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLengthFast_ThreeDigit(t *testing.T) {
	engine := fastrand.NewEngine(fastrand.WithMaxLength(500))

	for i := 0; i < 200; i++ {
		result := engine.RandomizerString("{RAND;100;ABL}")
		assert.Len(t, result, 100, "3-digit length 100 should produce 100 chars")
	}

	result := engine.RandomizerString("{RAND;255;ABL}")
	assert.Len(t, result, 255, "3-digit length 255 should produce 255 chars")
}

func TestParseLengthFast_ThreeDigitRange(t *testing.T) {
	engine := fastrand.NewEngine(fastrand.WithMinLength(1), fastrand.WithMaxLength(500))
	for i := 0; i < 200; i++ {
		result := engine.RandomizerString("{RAND;100-200;ABL}")
		l := len(result)
		assert.GreaterOrEqual(t, l, 100, "range 100-200 should produce >= 100")
		assert.LessOrEqual(t, l, 200, "range 100-200 should produce <= 200")
	}
}

func TestShouldEscape_TableConsistency(t *testing.T) {
	engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingURL))

	payloads := []string{
		"hello world",
		"a+b=c",
		"100% sure",
		"path/to/file",
		"key=value&foo=bar",
		"<html>",
		"\"quoted\"",
	}

	for _, p := range payloads {
		result := engine.RandomizerString(p)
		assert.NotEmpty(t, result, "should produce output for %q", p)
	}
}

func TestBytes_ZeroLen_Sentinel(t *testing.T) {
	b1 := fastrand.Bytes(0)
	b2 := fastrand.Bytes(0)
	assert.Equal(t, 0, len(b1))
	assert.Equal(t, 0, len(b2))
	assert.True(t, cap(b1) == 0 && cap(b2) == 0, "zero-length Bytes should return shared empty sentinel")
}

func TestSecureBytes_ZeroLen_Sentinel(t *testing.T) {
	b1, err := fastrand.SecureBytes(0)
	require.NoError(t, err)
	b2, err := fastrand.SecureBytes(0)
	require.NoError(t, err)
	assert.Equal(t, 0, len(b1))
	assert.Equal(t, 0, len(b2))
}

func TestRandomizerAppendString(t *testing.T) {
	engine := fastrand.NewEngine()
	dst := make([]byte, 0, 512)

	dst = engine.RandomizerAppendString(dst, "hello {RAND;8;ABL} world")
	assert.Contains(t, string(dst), "hello ")
	assert.Contains(t, string(dst), " world")
	assert.Greater(t, len(dst), len("hello  world"))
}

func TestRandomizerAppendString_NoTags(t *testing.T) {
	engine := fastrand.NewEngine()
	dst := make([]byte, 0, 64)

	dst = engine.RandomizerAppendString(dst, "plain text no tags")
	assert.Equal(t, "plain text no tags", string(dst))
}

func TestRandomizerAppendString_PreservesExisting(t *testing.T) {
	engine := fastrand.NewEngine()
	dst := []byte("prefix-")
	dst = engine.RandomizerAppendString(dst, "{RAND;4;DIGIT}")
	result := string(dst)
	assert.True(t, len(result) >= len("prefix-")+4, "should have prefix + 4 digits")
	assert.True(t, result[:len("prefix-")] == "prefix-", "should preserve prefix")
}

func TestS2B_RandomizerStringConsistency(t *testing.T) {
	engine := fastrand.NewEngine()
	payload := "test {RAND;10;ABL} data"

	s1 := engine.RandomizerString(payload)
	dst := engine.RandomizerAppendString(make([]byte, 0, 512), payload)
	s2 := string(dst)

	assert.Equal(t, len(s1), len(s2), "RandomizerString and RandomizerAppendString should produce same length")
	assert.Contains(t, s2, "test ")
	assert.Contains(t, s2, " data")
}

func TestAllocsRandomizerAppendString(t *testing.T) {
	engine := fastrand.NewEngine()
	payload := "hello {RAND;16;ABL} test {RAND;8;DIGIT} end"
	dst := make([]byte, 0, 512)

	allocs := testing.AllocsPerRun(100, func() {
		dst = dst[:0]
		dst = engine.RandomizerAppendString(dst, payload)
	})

	if allocs > 1 {
		t.Errorf("RandomizerAppendString allocated %v times, expected <= 1", allocs)
	}
}

func TestNormalizeString_UrlDecoding(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL),
		fastrand.WithOutputEncoding(fastrand.RandomizerEncodingNone),
	)
	result := engine.RandomizerString("%7BRAND;8;ABL%7D")
	if len(result) != 8 {
		t.Errorf("URL-decoded {RAND;8;ABL} should produce 8 chars, got %d: %q", len(result), result)
	}
}

func TestNormalizeString_HtmlDecoding(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithInputEncoding(fastrand.RandomizerEncodingHTML),
		fastrand.WithOutputEncoding(fastrand.RandomizerEncodingNone),
	)
	result := engine.RandomizerString("&lbrace;RAND;8;ABL&rbrace;")
	if len(result) != 8 {
		t.Errorf("HTML-decoded {RAND;8;ABL} should produce 8 chars, got %d: %q", len(result), result)
	}
}

func TestStrconvPutUint_DigitTable(t *testing.T) {
	engine := fastrand.NewEngine()
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;1;IPV4}")
		if len(result) < 7 {
			t.Errorf("IPv4 should be at least 7 chars (0.0.0.0), got %q", result)
		}
	}
}

func TestAppendIPv4_Format(t *testing.T) {
	engine := fastrand.NewEngine()
	for i := 0; i < 200; i++ {
		result := engine.RandomizerString("{RAND;1;IPV4}")
		dots := 0
		for _, c := range result {
			if c == '.' {
				dots++
			}
		}
		if dots != 3 {
			t.Errorf("IPv4 should have 3 dots, got %d in %q", dots, result)
		}
	}
}

func TestParseAndReplaceFast_LengthChoices(t *testing.T) {
	engine := fastrand.NewEngine()
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;5,10,15;ABL}")
		l := len(result)
		if l != 5 && l != 10 && l != 15 {
			t.Errorf("length choices should be 5/10/15, got %d: %q", l, result)
		}
	}
}

func TestParseAndReplaceFast_KeywordChoices(t *testing.T) {
	engine := fastrand.NewEngine()
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;4;ABL,ABU}")
		if len(result) != 4 {
			t.Errorf("keyword choice should produce 4 chars, got %d: %q", len(result), result)
		}
	}
}

func TestAppendIPv6_Format(t *testing.T) {
	engine := fastrand.NewEngine()
	for i := 0; i < 200; i++ {
		result := engine.RandomizerString("{RAND;1;IPV6}")
		colons := 0
		for _, c := range result {
			if c == ':' {
				colons++
			}
		}
		if colons != 7 {
			t.Errorf("IPv6 should have 7 colons, got %d in %q", colons, result)
		}
		for _, c := range result {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || c == ':'
			if !isHex {
				t.Errorf("IPv6 should only contain hex chars and colons, got %q", result)
				break
			}
		}
	}
}

func TestAppendURLEncode_SpaceAndUnsafe(t *testing.T) {
	engine := fastrand.NewEngine(fastrand.WithOutputEncoding(fastrand.RandomizerEncodingURL))
	result := engine.RandomizerString("hello world<test>")
	if !strings.Contains(result, "+") {
		t.Errorf("space should be encoded as +, got %q", result)
	}
	if !strings.Contains(result, "%3C") {
		t.Errorf("< should be percent-encoded, got %q", result)
	}
}

func TestFillStringInto_NonPowerOfTwo(t *testing.T) {
	charset := fastrand.CharsList("abcdef")
	buf := make([]byte, 1000)
	fastrand.FillString(buf, charset)
	for _, c := range buf {
		if c < 'a' || c > 'f' {
			t.Errorf("char should be in [a-f], got %c", c)
			break
		}
	}
}

func TestSPACE_Keyword(t *testing.T) {
	engine := fastrand.NewEngine()
	result := engine.RandomizerString("{RAND;20;SPACE}")
	if len(result) != 20 {
		t.Fatalf("SPACE should produce 20 chars, got %d", len(result))
	}
	for _, c := range result {
		if c != ' ' {
			t.Errorf("SPACE should produce only spaces, got %c in %q", c, result)
			break
		}
	}
}

func TestNULL_Keyword(t *testing.T) {
	engine := fastrand.NewEngine()
	result := engine.RandomizerString("{RAND;16;NULL}")
	if len(result) != 16 {
		t.Fatalf("NULL should produce 16 chars, got %d", len(result))
	}
	for _, c := range result {
		if c > 15 {
			t.Errorf("NULL should produce bytes 0-15, got %d in %q", c, result)
			break
		}
	}
}

func TestBytesEqualFold_CaseInsensitive(t *testing.T) {
	engine := fastrand.NewEngine()
	r1 := engine.RandomizerString("{RAND;8;abl}")
	r2 := engine.RandomizerString("{RAND;8;ABL}")
	if len(r1) != 8 || len(r2) != 8 {
		t.Errorf("both should produce 8 chars, got %d and %d", len(r1), len(r2))
	}
}

func TestCustomKeyword_DedupLookup(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithCustomKeyword("MYKW", func(length int) []byte {
			return []byte("custom_output")
		}),
		fastrand.WithDisabledKeywords("ABL"),
	)
	result := engine.RandomizerString("{RAND;5;MYKW}")
	if result != "custom_output" {
		t.Errorf("custom keyword should produce 'custom_output', got %q", result)
	}
	result2 := engine.RandomizerString("{RAND;5;abl}")
	if len(result2) != 5 {
		t.Errorf("disabled ABL should fall back to default, got len %d", len(result2))
	}
}

func TestKeywordDispatch_MixedCase(t *testing.T) {
	engine := fastrand.NewEngine()
	r1 := engine.RandomizerString("{RAND;4;AbL}")
	r2 := engine.RandomizerString("{RAND;4;ABl}")
	if len(r1) != 4 || len(r2) != 4 {
		t.Errorf("mixed case keywords should work, got %d, %d", len(r1), len(r2))
	}
	for _, c := range r1 {
		if c < 'a' || c > 'z' {
			t.Errorf("AbL should produce lowercase, got %c", c)
			break
		}
	}
	for _, c := range r2 {
		if c < 'a' || c > 'z' {
			t.Errorf("ABl should produce lowercase, got %c", c)
			break
		}
	}
}
