package fastrand_test

import (
	"testing"

	"github.com/obeliskdev/fastrand"
)

func TestAllocsRandomizerString(t *testing.T) {
	engine := fastrand.NewEngine()
	payload := "hello world {RAND;16;ABL} test {RAND;8;DIGIT} end"

	allocs := testing.AllocsPerRun(100, func() {
		_ = engine.RandomizerString(payload)
	})

	if allocs > 3 {
		t.Errorf("RandomizerString allocated %v times, expected <= 3", allocs)
	}
}

func TestAllocsRandomizerStringNoTags(t *testing.T) {
	engine := fastrand.NewEngine()
	payload := "hello world no tags here"

	allocs := testing.AllocsPerRun(100, func() {
		_ = engine.RandomizerString(payload)
	})

	if allocs > 0 {
		t.Errorf("RandomizerString with no tags allocated %v times, expected 0", allocs)
	}
}

func TestAllocsRandomizerAppend(t *testing.T) {
	engine := fastrand.NewEngine()
	payload := []byte("hello world {RAND;16;ABL} test {RAND;8;DIGIT} end")
	dst := make([]byte, 0, 512)

	allocs := testing.AllocsPerRun(100, func() {
		dst = dst[:0]
		dst = engine.RandomizerAppend(dst, payload)
	})

	if allocs > 1 {
		t.Errorf("RandomizerAppend allocated %v times, expected <= 1", allocs)
	}
}

func TestAllocsReset(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithCustomCharset("CUSTOM", []byte("ABC")),
		fastrand.WithDisabledKeywords("UUID"),
	)

	allocs := testing.AllocsPerRun(100, func() {
		engine.Reset()
	})

	if allocs > 0 {
		t.Errorf("Reset allocated %v times, expected 0", allocs)
	}
}

func TestAllocsNormalizeNoEncodingNeeded(t *testing.T) {
	engine := fastrand.NewEngine(fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL))

	payload := "plain text with no special chars"

	allocs := testing.AllocsPerRun(100, func() {
		_ = engine.RandomizerString(payload)
	})

	if allocs > 1 {
		t.Errorf("RandomizerString with no encoding needed allocated %v times, expected <= 1", allocs)
	}
}

func TestAllocsFillString(t *testing.T) {
	buf := make([]byte, 64)
	charset := fastrand.CharsAlphabetLower

	allocs := testing.AllocsPerRun(100, func() {
		fastrand.FillString(buf, charset)
	})

	if allocs > 0 {
		t.Errorf("FillString allocated %v times, expected 0", allocs)
	}
}

func TestAllocsFillHex(t *testing.T) {
	buf := make([]byte, 64)

	allocs := testing.AllocsPerRun(100, func() {
		fastrand.FillHex(buf)
	})

	if allocs > 0 {
		t.Errorf("FillHex allocated %v times, expected 0", allocs)
	}
}

func TestAllocsFillBytes(t *testing.T) {
	buf := make([]byte, 64)

	allocs := testing.AllocsPerRun(100, func() {
		fastrand.FillBytes(buf)
	})

	if allocs > 0 {
		t.Errorf("FillBytes allocated %v times, expected 0", allocs)
	}
}
