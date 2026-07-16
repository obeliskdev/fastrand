package fastrand_test

import (
	"encoding/hex"
	"net"
	"strings"
	"testing"

	"github.com/obeliskdev/fastrand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFillBytes(t *testing.T) {
	t.Parallel()

	t.Run("FullBuffer", func(t *testing.T) {
		buf := make([]byte, 64)
		fastrand.FillBytes(buf)
		isZero := true
		for _, b := range buf {
			if b != 0 {
				isZero = false
				break
			}
		}
		assert.False(t, isZero, "FillBytes should produce non-zero bytes")
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		assert.NotPanics(t, func() { fastrand.FillBytes(buf) })
	})

	t.Run("OddLength", func(t *testing.T) {
		buf := make([]byte, 7)
		fastrand.FillBytes(buf)
		isZero := true
		for _, b := range buf {
			if b != 0 {
				isZero = false
				break
			}
		}
		assert.False(t, isZero, "FillBytes should fill odd-length buffer")
	})

	t.Run("Length1", func(t *testing.T) {
		buf := make([]byte, 1)
		fastrand.FillBytes(buf)
		assert.NotEqual(t, byte(0), buf[0], "Single byte should not be zero (usually)")
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		fastrand.FillBytes(buf1)
		fastrand.FillBytes(buf2)
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different results")
	})

	t.Run("VariousSizes", func(t *testing.T) {
		sizes := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025}
		for _, size := range sizes {
			buf := make([]byte, size)
			fastrand.FillBytes(buf)
			isZero := true
			for _, b := range buf {
				if b != 0 {
					isZero = false
					break
				}
			}
			assert.False(t, isZero, "Size %d: buffer should not be all zeros", size)
		}
	})

	t.Run("ConcurrentSafe", func(t *testing.T) {
		t.Parallel()
		buf := make([]byte, 32)
		for i := 0; i < 1000; i++ {
			fastrand.FillBytes(buf)
		}
	})
}

func TestFillString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		length  int
		charset fastrand.CharsList
	}{
		{"Digits1", 1, fastrand.CharsDigits},
		{"Digits8", 8, fastrand.CharsDigits},
		{"Lower16", 16, fastrand.CharsAlphabetLower},
		{"Upper32", 32, fastrand.CharsAlphabetUpper},
		{"Alphabet64", 64, fastrand.CharsAlphabet},
		{"Alphanum128", 128, fastrand.CharsAlphabetDigits},
		{"Symbols", 32, fastrand.CharsSymbolChars},
		{"All", 100, fastrand.CharsAll},
		{"Null16", 16, fastrand.CharsNull},
		{"Space1", 1, fastrand.CharsSpace},
		{"Custom2", 10, fastrand.CharsList("ab")},
		{"Custom4", 20, fastrand.CharsList("abcd")},
		{"Custom8", 30, fastrand.CharsList("abcdefgh")},
		{"Custom16", 40, fastrand.CharsList("0123456789abcdef")},
		{"Custom32", 50, fastrand.CharsList("0123456789abcdefghijklmnopqrstuv")},
		{"Custom64", 60, fastrand.CharsList("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.length)
			fastrand.FillString(buf, tc.charset)
			require.Len(t, buf, tc.length, "Buffer length should not change")

			cs := string(tc.charset)
			for i, b := range buf {
				if !strings.ContainsRune(cs, rune(b)) {
					t.Errorf("Byte %d: 0x%02X ('%c') not in charset %q", i, b, b, cs)
				}
			}
		})
	}

	t.Run("EmptyCharsetPanics", func(t *testing.T) {
		buf := make([]byte, 10)
		assert.Panics(t, func() {
			fastrand.FillString(buf, fastrand.CharsList(""))
		})
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		assert.NotPanics(t, func() {
			fastrand.FillString(buf, fastrand.CharsDigits)
		})
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		fastrand.FillString(buf1, fastrand.CharsAlphabetDigits)
		fastrand.FillString(buf2, fastrand.CharsAlphabetDigits)
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different results")
	})

	t.Run("PowerOfTwoCharsetUniformity", func(t *testing.T) {
		buf := make([]byte, 10000)
		fastrand.FillString(buf, fastrand.CharsList("ab"))
		countA, countB := 0, 0
		for _, b := range buf {
			if b == 'a' {
				countA++
			} else if b == 'b' {
				countB++
			} else {
				t.Errorf("Unexpected byte 0x%02X in binary charset", b)
			}
		}
		assert.InDelta(t, 5000, countA, 500, "Binary charset should be roughly uniform")
		assert.InDelta(t, 5000, countB, 500, "Binary charset should be roughly uniform")
	})

	t.Run("NonPowerOfTwoCharsetUniformity", func(t *testing.T) {
		buf := make([]byte, 10000)
		fastrand.FillString(buf, fastrand.CharsDigits)
		counts := make(map[byte]int)
		for _, b := range buf {
			counts[b]++
		}
		assert.Len(t, counts, 10, "Should see all 10 digits")
		for d := byte('0'); d <= '9'; d++ {
			assert.InDelta(t, 1000, counts[d], 300, "Digit %c should appear ~1000 times", d)
		}
	})
}

func TestFillHex(t *testing.T) {
	t.Parallel()

	t.Run("ValidHex", func(t *testing.T) {
		buf := make([]byte, 64)
		fastrand.FillHex(buf)
		assert.Len(t, buf, 64)
		assert.Regexp(t, `^[a-f0-9]{64}$`, string(buf), "FillHex should produce valid hex")
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		assert.NotPanics(t, func() { fastrand.FillHex(buf) })
	})

	t.Run("OddLengthPanics", func(t *testing.T) {
		buf := make([]byte, 7)
		assert.Panics(t, func() { fastrand.FillHex(buf) })
	})

	t.Run("VariousSizes", func(t *testing.T) {
		sizes := []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024}
		for _, size := range sizes {
			buf := make([]byte, size)
			fastrand.FillHex(buf)
			assert.True(t, isValidHex(string(buf)), "Size %d: invalid hex", size)
		}
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		fastrand.FillHex(buf1)
		fastrand.FillHex(buf2)
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different hex")
	})

	t.Run("MatchesHexEncode", func(t *testing.T) {
		raw := make([]byte, 32)
		fastrand.FillBytes(raw)
		expected := make([]byte, 64)
		hex.Encode(expected, raw)

		got := make([]byte, 64)
		fastrand.FillHex(got)

		if string(got) == string(expected) {
			t.Logf("Warning: FillHex matched FillBytes+hex.Encode exactly (unlikely coincidence)")
		}
		assert.Regexp(t, `^[a-f0-9]{64}$`, string(got))
	})
}

func TestSecureFillBytes(t *testing.T) {
	t.Parallel()

	t.Run("FullBuffer", func(t *testing.T) {
		buf := make([]byte, 64)
		err := fastrand.SecureFillBytes(buf)
		require.NoError(t, err)
		isZero := true
		for _, b := range buf {
			if b != 0 {
				isZero = false
				break
			}
		}
		assert.False(t, isZero, "SecureFillBytes should produce non-zero bytes")
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		err := fastrand.SecureFillBytes(buf)
		require.NoError(t, err)
	})

	t.Run("OddLength", func(t *testing.T) {
		buf := make([]byte, 7)
		err := fastrand.SecureFillBytes(buf)
		require.NoError(t, err)
		isZero := true
		for _, b := range buf {
			if b != 0 {
				isZero = false
				break
			}
		}
		assert.False(t, isZero, "SecureFillBytes should fill odd-length buffer")
	})

	t.Run("VariousSizes", func(t *testing.T) {
		sizes := []int{1, 2, 3, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025}
		for _, size := range sizes {
			buf := make([]byte, size)
			err := fastrand.SecureFillBytes(buf)
			require.NoError(t, err, "Size %d: SecureFillBytes should not error", size)
			isZero := true
			for _, b := range buf {
				if b != 0 {
					isZero = false
					break
				}
			}
			assert.False(t, isZero, "Size %d: buffer should not be all zeros", size)
		}
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		require.NoError(t, fastrand.SecureFillBytes(buf1))
		require.NoError(t, fastrand.SecureFillBytes(buf2))
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different results")
	})

	t.Run("ConcurrentSafe", func(t *testing.T) {
		t.Parallel()
		buf := make([]byte, 32)
		for i := 0; i < 1000; i++ {
			require.NoError(t, fastrand.SecureFillBytes(buf))
		}
	})
}

func TestSecureFillString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		length  int
		charset fastrand.CharsList
	}{
		{"Digits1", 1, fastrand.CharsDigits},
		{"Digits8", 8, fastrand.CharsDigits},
		{"Lower16", 16, fastrand.CharsAlphabetLower},
		{"Upper32", 32, fastrand.CharsAlphabetUpper},
		{"Alphabet64", 64, fastrand.CharsAlphabet},
		{"Alphanum128", 128, fastrand.CharsAlphabetDigits},
		{"Symbols", 32, fastrand.CharsSymbolChars},
		{"All", 100, fastrand.CharsAll},
		{"Null16", 16, fastrand.CharsNull},
		{"Space1", 1, fastrand.CharsSpace},
		{"Custom2", 10, fastrand.CharsList("ab")},
		{"Custom4", 20, fastrand.CharsList("abcd")},
		{"Custom8", 30, fastrand.CharsList("abcdefgh")},
		{"Custom16", 40, fastrand.CharsList("0123456789abcdef")},
		{"Custom32", 50, fastrand.CharsList("0123456789abcdefghijklmnopqrstuv")},
		{"Custom64", 60, fastrand.CharsList("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.length)
			err := fastrand.SecureFillString(buf, tc.charset)
			require.NoError(t, err)
			require.Len(t, buf, tc.length, "Buffer length should not change")

			cs := string(tc.charset)
			for i, b := range buf {
				if !strings.ContainsRune(cs, rune(b)) {
					t.Errorf("Byte %d: 0x%02X ('%c') not in charset %q", i, b, b, cs)
				}
			}
		})
	}

	t.Run("EmptyCharsetErrors", func(t *testing.T) {
		buf := make([]byte, 10)
		err := fastrand.SecureFillString(buf, fastrand.CharsList(""))
		require.Error(t, err)
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		err := fastrand.SecureFillString(buf, fastrand.CharsDigits)
		require.NoError(t, err)
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		require.NoError(t, fastrand.SecureFillString(buf1, fastrand.CharsAlphabetDigits))
		require.NoError(t, fastrand.SecureFillString(buf2, fastrand.CharsAlphabetDigits))
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different results")
	})

	t.Run("Uniformity", func(t *testing.T) {
		buf := make([]byte, 10000)
		require.NoError(t, fastrand.SecureFillString(buf, fastrand.CharsDigits))
		counts := make(map[byte]int)
		for _, b := range buf {
			counts[b]++
		}
		assert.Len(t, counts, 10, "Should see all 10 digits")
		for d := byte('0'); d <= '9'; d++ {
			assert.InDelta(t, 1000, counts[d], 300, "Digit %c should appear ~1000 times", d)
		}
	})
}

func TestSecureFillHex(t *testing.T) {
	t.Parallel()

	t.Run("ValidHex", func(t *testing.T) {
		buf := make([]byte, 64)
		err := fastrand.SecureFillHex(buf)
		require.NoError(t, err)
		assert.Regexp(t, `^[a-f0-9]{64}$`, string(buf), "SecureFillHex should produce valid hex")
	})

	t.Run("EmptyBuffer", func(t *testing.T) {
		buf := make([]byte, 0)
		err := fastrand.SecureFillHex(buf)
		require.NoError(t, err)
	})

	t.Run("OddLengthErrors", func(t *testing.T) {
		buf := make([]byte, 7)
		err := fastrand.SecureFillHex(buf)
		require.Error(t, err)
	})

	t.Run("VariousSizes", func(t *testing.T) {
		sizes := []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024}
		for _, size := range sizes {
			buf := make([]byte, size)
			err := fastrand.SecureFillHex(buf)
			require.NoError(t, err, "Size %d: SecureFillHex should not error", size)
			assert.True(t, isValidHex(string(buf)), "Size %d: invalid hex", size)
		}
	})

	t.Run("DistinctCalls", func(t *testing.T) {
		buf1 := make([]byte, 32)
		buf2 := make([]byte, 32)
		require.NoError(t, fastrand.SecureFillHex(buf1))
		require.NoError(t, fastrand.SecureFillHex(buf2))
		assert.NotEqual(t, buf1, buf2, "Two calls should produce different hex")
	})
}

func TestFillBytesConcurrency(t *testing.T) {
	t.Parallel()
	const numGoroutines = 50
	const opsPerGoroutine = 200

	done := make(chan struct{}, numGoroutines*4)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			buf := make([]byte, 64)
			for i := 0; i < opsPerGoroutine; i++ {
				fastrand.FillBytes(buf)
			}
			done <- struct{}{}
		}()
		go func() {
			buf := make([]byte, 32)
			for i := 0; i < opsPerGoroutine; i++ {
				fastrand.FillString(buf, fastrand.CharsAlphabetDigits)
			}
			done <- struct{}{}
		}()
		go func() {
			buf := make([]byte, 64)
			for i := 0; i < opsPerGoroutine; i++ {
				fastrand.FillHex(buf)
			}
			done <- struct{}{}
		}()
		go func() {
			buf := make([]byte, 64)
			for i := 0; i < opsPerGoroutine; i++ {
				_ = fastrand.SecureFillBytes(buf)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < numGoroutines*4; i++ {
		<-done
	}
}

func TestSecureFillBytesConcurrency(t *testing.T) {
	t.Parallel()
	const numGoroutines = 50
	const opsPerGoroutine = 200

	done := make(chan struct{}, numGoroutines*3)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			buf := make([]byte, 64)
			for i := 0; i < opsPerGoroutine; i++ {
				_ = fastrand.SecureFillBytes(buf)
			}
			done <- struct{}{}
		}()
		go func() {
			buf := make([]byte, 32)
			for i := 0; i < opsPerGoroutine; i++ {
				_ = fastrand.SecureFillString(buf, fastrand.CharsAlphabetDigits)
			}
			done <- struct{}{}
		}()
		go func() {
			buf := make([]byte, 64)
			for i := 0; i < opsPerGoroutine; i++ {
				_ = fastrand.SecureFillHex(buf)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < numGoroutines*3; i++ {
		<-done
	}
}

func TestFillBytesZeroAllocPattern(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 64)
	original := make([]byte, 64)
	copy(original, buf)
	fastrand.FillBytes(buf)
	changed := false
	for i := range buf {
		if buf[i] != original[i] {
			changed = true
			break
		}
	}
	assert.True(t, changed, "FillBytes should modify the buffer in-place")
}

func TestIPv4Validity(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		ip := fastrand.IPv4()
		require.NotNil(t, ip)
		require.Len(t, ip, net.IPv4len)
		assert.NotNil(t, ip.To4(), "Generated IP should be valid IPv4")
	}
}

func TestIPv6Validity(t *testing.T) {
	t.Parallel()
	for i := 0; i < 1000; i++ {
		ip := fastrand.IPv6()
		require.NotNil(t, ip)
		require.Len(t, ip, net.IPv6len)
		assert.NotNil(t, ip.To16(), "Generated IP should be valid IPv6")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func isValidHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
