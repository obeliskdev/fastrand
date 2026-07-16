package fastrand_test

import (
	"testing"

	"github.com/obeliskdev/fastrand"
	"github.com/stretchr/testify/assert"
)

func TestUint16N(t *testing.T) {
	t.Parallel()
	for i := 0; i < numTestIterations; i++ {
		v := fastrand.Uint16N(1000)
		assert.Less(t, v, uint16(1000), "Uint16N(1000) = %d, should be < 1000", v)
	}
}

func TestUint16NRange(t *testing.T) {
	t.Parallel()
	seen := make(map[uint16]bool)
	for i := 0; i < 100000; i++ {
		v := fastrand.Uint16N(8)
		assert.Less(t, v, uint16(8))
		seen[v] = true
	}
	// With 100K draws from [0,8), all 8 values should appear.
	assert.Len(t, seen, 8, "Uint16N(8) should produce all 8 values in 100K draws")
}

func TestUint16NPowerOfTwo(t *testing.T) {
	t.Parallel()
	for i := 0; i < numTestIterations; i++ {
		v := fastrand.Uint16N(256)
		assert.Less(t, v, uint16(256))
	}
}

func TestUint16NPanicsOnZero(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { fastrand.Uint16N(0) })
}

func TestUint16NOne(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		assert.Equal(t, uint16(0), fastrand.Uint16N(1))
	}
}

func TestUint8N(t *testing.T) {
	t.Parallel()
	for i := 0; i < numTestIterations; i++ {
		v := fastrand.Uint8N(200)
		assert.Less(t, v, uint8(200), "Uint8N(200) = %d, should be < 200", v)
	}
}

func TestUint8NRange(t *testing.T) {
	t.Parallel()
	seen := make(map[uint8]bool)
	for i := 0; i < 100000; i++ {
		v := fastrand.Uint8N(7)
		assert.Less(t, v, uint8(7))
		seen[v] = true
	}
	assert.Len(t, seen, 7, "Uint8N(7) should produce all 7 values in 100K draws")
}

func TestUint8NPowerOfTwo(t *testing.T) {
	t.Parallel()
	for i := 0; i < numTestIterations; i++ {
		v := fastrand.Uint8N(16)
		assert.Less(t, v, uint8(16))
	}
}

func TestUint8NPanicsOnZero(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { fastrand.Uint8N(0) })
}

func TestUint8NOne(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		assert.Equal(t, uint8(0), fastrand.Uint8N(1))
	}
}

func TestUint64(t *testing.T) {
	t.Parallel()
	seen := make(map[uint64]bool)
	for i := 0; i < 1000; i++ {
		v := fastrand.Uint64()
		assert.False(t, seen[v], "Uint64() produced duplicate %d at iteration %d", v, i)
		seen[v] = true
	}
}

func BenchmarkUint64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fastrand.Uint64()
	}
}

func BenchmarkUint16N(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fastrand.Uint16N(65535)
	}
}

func BenchmarkUint8N(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fastrand.Uint8N(223)
	}
}

func BenchmarkIntForPortRange(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fastrand.Int(1, 65535)
	}
}
