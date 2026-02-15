package fastrand_test

import (
	crand "crypto/rand"
	"fmt"
	"github.com/obeliskdev/fastrand"
	"testing"
)

var benchmarkErr error

func BenchmarkIntnFastRand(b *testing.B) {
	b.ReportAllocs()
	var res int
	for i := 0; i < b.N; i++ {
		res = fastrand.IntN(10000)
	}
	_ = res
}

func BenchmarkSecureIntFastRand(b *testing.B) {
	b.ReportAllocs()
	var res int
	var err error
	for i := 0; i < b.N; i++ {
		res, err = fastrand.SecureInt(0, 10000-1)
	}
	_ = res
	benchmarkErr = err
}

func BenchmarkFloat64FastRand(b *testing.B) {
	b.ReportAllocs()
	var res float64
	for i := 0; i < b.N; i++ {
		res = fastrand.Float64()
	}
	_ = res
}

func BenchmarkSecureFloat64FastRand(b *testing.B) {
	b.ReportAllocs()
	var res float64
	for i := 0; i < b.N; i++ {
		res = fastrand.SecureFloat64()
	}
	_ = res
}

var byteBenchmarkSizes = []int{8, 64, 512, 4096}

func BenchmarkInternalFastBytes(b *testing.B) {
	for _, size := range byteBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			buf := make([]byte, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {

				filled := 0
				for filled < size {

					randValF := fastrand.Float64()
					randVal := uint64(randValF * float64(1<<63))

					bytesToCopy := 8
					remaining := size - filled
					if remaining < bytesToCopy {
						bytesToCopy = remaining
					}

					if bytesToCopy > 0 {
						buf[filled] = byte(randVal)
					}
					filled += bytesToCopy
				}

			}
			_ = buf
		})
	}
}

func BenchmarkBytesCryptoRand(b *testing.B) {
	for _, size := range byteBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			var res []byte
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res = fastrand.Bytes(size)
			}
			_ = res
		})
	}
}

func BenchmarkSecureBytesFastRand(b *testing.B) {
	for _, size := range byteBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			var res []byte
			var err error
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res, err = fastrand.SecureBytes(size)
			}
			_ = res
			benchmarkErr = err
		})
	}
}

func BenchmarkBytesStdLibCryptoRand(b *testing.B) {
	for _, size := range byteBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			buf := make([]byte, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, benchmarkErr = crand.Read(buf)
				if benchmarkErr != nil {
					b.Fatal(benchmarkErr)
				}
			}
			_ = buf
		})
	}
}

var stringBenchmarkSizes = []int{8, 32, 128, 1024}
var stringCharset = fastrand.CharsAlphabetDigits

func BenchmarkStringFastRand(b *testing.B) {
	cs := string(stringCharset)
	for _, size := range stringBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			var res string
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res = fastrand.String(size, fastrand.CharsList(cs))
			}
			_ = res
		})
	}
}

func BenchmarkSecureStringFastRand(b *testing.B) {
	cs := string(stringCharset)
	for _, size := range stringBenchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			var res string
			var err error
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res, err = fastrand.SecureString(size, fastrand.CharsList(cs))
			}
			_ = res
			benchmarkErr = err
		})
	}
}

func BenchmarkEngine(b *testing.B) {
	engine := fastrand.NewEngine()
	payload := []byte("User:{RAND;10-20;ABL,ABU}|Sess:{RAND;32;HEX}|ID:{RAND;UUID,HEX}|IP:{RAND;IPV4}|Data:{RAND;50,60,70}")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Randomizer(payload)
	}
}

func BenchmarkRandomizer(b *testing.B) {
	payload := []byte("User: {RAND;10-20;ABL,ABU} | Session: {RANDOM;32;HEX} | ID: {RAND;UUID,HEX} | IP: {RAND;IPV4} | Data: {RAND;50-99} --- End")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fastrand.Randomizer(payload)
	}
}
