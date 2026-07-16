# FastRand — High-Performance Random Data Generator for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/obeliskdev/fastrand.svg)](https://pkg.go.dev/github.com/obeliskdev/fastrand)
[![Go Report Card](https://goreportcard.com/badge/github.com/obeliskdev/fastrand)](https://goreportcard.com/report/github.com/obeliskdev/fastrand)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

FastRand is a high-performance, zero-allocation random data generation library for Go. It combines fast non-cryptographic randomness, cryptographically secure randomness, and a template-driven randomizer engine into a single package — optimized for simulations, fuzz testing, synthetic data generation, token generation, and load testing.

## Table of Contents

- [Features](#features)
- [Performance](#performance)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core API](#core-api)
  - [Numeric](#numeric)
  - [Secure Numeric](#secure-numeric)
  - [Bytes and Strings](#bytes-and-strings)
  - [Zero-Allocation Fill APIs](#zero-allocation-fill-apis)
  - [Collections](#collections)
  - [Network and IDs](#network-and-ids)
- [Randomizer Engine](#randomizer-engine)
  - [Placeholder Syntax](#placeholder-syntax)
  - [Keywords](#keywords)
  - [Length Specification](#length-specification)
  - [Keyword Choices](#keyword-choices)
  - [URL/HTML Encoding](#urlhtml-encoding)
  - [Engine Options](#engine-options)
  - [RandomizerAppend — Zero-Allocation Output](#randomizerappend--zero-allocation-output)
- [Concurrency](#concurrency)
- [Testing](#testing)
- [Benchmarks](#benchmarks)
- [License](#license)

## Features

- **Dual RNG strategy**: lock-free splitmix64 for fast path, ChaCha8 for secure path
- **Zero-allocation fill APIs**: `FillBytes`, `FillString`, `FillHex`, `SecureFillBytes`, `SecureFillString`, `SecureFillHex` — write random data directly into caller-provided buffers with zero heap allocations
- **Template-driven randomizer**: generate structured synthetic data from `{RAND;length;keyword}` placeholders with support for length ranges, keyword choices, custom keywords, custom charsets, and URL/HTML encoding
- **Generic numeric helpers**: `Number[T]` and `SecureNumber[T]` work across all integer and float types
- **Case-insensitive keywords**: `{RAND;8;digit}`, `{RAND;8;Digit}`, `{RAND;8;DIGIT}` are all equivalent
- **Thread-safe**: all package-level functions and engine methods are safe for concurrent use
- **No external dependencies**: only Go standard library (test-only deps excluded)
- **UUID v4 generation**: RFC 4122 compliant UUIDs via `FastUUID`/`SecureUUID`
- **IPv4/IPv6 generation**: random IP addresses with string formatting support

## Performance

FastRand is engineered for minimal allocations and maximum throughput:

| Benchmark | Allocations | Bytes | Notes |
|---|---|---|---|
| `IntN(10000)` | 0 | 0 | Lock-free splitmix64 |
| `Float64()` | 0 | 0 | 53-bit mantissa |
| `String(32, CharsAlphabetDigits)` | 1 | 32 | Unavoidable result allocation |
| `FillString(buf, charset)` | **0** | **0** | Zero-alloc fill into caller buffer |
| `FillHex(buf)` | **0** | **0** | Zero-alloc hex fill |
| `SecureFillBytes(buf)` | **0** | **0** | Single-lock ChaCha8 batching |
| `RandomizerString(template)` | **1** | 640 | Single result buffer |
| `RandomizerAppend(dst, payload)` | **1** | 512 | Single caller buffer |
| `SecureBytes(4096)` | 1 | 4096 | 2.8x faster than previous (single-lock) |

Key optimizations:
- splitmix64 with `atomic.Uint64.Add` — fully lock-free fast path
- Power-of-two charset fast path: 8 indices per `fastUint64()` call
- Fisher-Yates shuffle/perm inlined — no per-call `rand.New` allocation
- `SecureFillBytes` batches 8-byte writes under a single mutex lock
- Randomizer engine: stack-based ASCII uppercasing, direct buffer writes, `bytesEqualFold` — eliminates all hot-path string allocations

## Installation

```bash
go get github.com/obeliskdev/fastrand
```

Requires Go 1.25+.

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/obeliskdev/fastrand"
)

func main() {
	// Fast random integers
	fmt.Println("IntN:", fastrand.IntN(100))
	fmt.Println("Range:", fastrand.Int(10, 20))
	fmt.Println("Bool:", fastrand.Bool())

	// Random strings and hex tokens
	fmt.Println("Hex token:", fastrand.Hex(16))
	fmt.Println("Password:", fastrand.String(24, fastrand.CharsAll))

	// Cryptographically secure random
	pwd, _ := fastrand.SecureString(24, fastrand.CharsAll)
	fmt.Println("Secure password:", pwd)

	// Template-driven synthetic data
	out := fastrand.RandomizerString("user={RAND;8;ABL}&pin={RAND;6;DIGIT}&id={RAND;UUID}")
	fmt.Println("Synthetic:", out)

	// Zero-allocation fill
	buf := make([]byte, 32)
	fastrand.FillString(buf, fastrand.CharsAlphabetDigits)
	fmt.Println("Zero-alloc string:", string(buf))
}
```

## Core API

### Numeric

- `Int(min, max int) int` — random integer in inclusive range [min, max]
- `IntN(n int) int` — random integer in [0, n)
- `Float64() float64` — random float in [0.0, 1.0)
- `Number[T number](min, max T) T` — generic numeric for any int/uint/float type
- `NumberN[T number](n T) T` — generic Number in [0, n]

### Secure Numeric

- `SecureInt(min, max int) (int, error)` — secure random integer in inclusive range
- `SecureIntN(n int) (int, error)` — secure random integer in [0, n)
- `SecureFloat64() float64` — secure random float in [0.0, 1.0)
- `SecureNumber[T number](min, max T) (T, error)` — generic secure numeric
- `SecureNumberN[T number](n T) (T, error)` — generic secure Number in [0, n]

### Bytes and Strings

- `Bytes(length int) []byte` — random bytes
- `String(length int, charset CharsList) string` — random string from charset
- `Hex(length int) string` — hex-encoded random string (length × 2 hex chars)
- `SecureBytes(length int) ([]byte, error)` — cryptographically secure random bytes
- `SecureString(length int, charset CharsList) (string, error)` — secure random string
- `SecureHex(length int) (string, error)` — secure hex-encoded string

**Predefined charsets:**

- `CharsAlphabetLower` — `abcdefghijklmnopqrstuvwxyz`
- `CharsAlphabetUpper` — `ABCDEFGHIJKLMNOPQRSTUVWXYZ`
- `CharsAlphabet` — lower + upper
- `CharsDigits` — `0123456789`
- `CharsAlphabetDigits` — alphabet + digits
- `CharsSymbolChars` — `!"#$%&'()*+,-./:;<=>?@[\]^_`{|}~`
- `CharsAll` — alphabet + digits + symbols
- `CharsNull` — bytes 0–15
- `CharsSpace` — single space

### Zero-Allocation Fill APIs

Write random data directly into a caller-provided buffer. **Zero heap allocations** — ideal for hot paths, connection pools, and high-throughput generators.

- `FillBytes(buf []byte)` — fill buffer with random bytes
- `FillString(buf []byte, charset CharsList)` — fill buffer with random chars from charset
- `FillHex(dst []byte)` — fill buffer with hex-encoded random bytes (dst length must be even)
- `SecureFillBytes(buf []byte) error` — fill with cryptographically secure random bytes
- `SecureFillString(buf []byte, charset CharsList) error` — fill with secure random chars
- `SecureFillHex(dst []byte) error` — fill with hex-encoded secure random bytes

```go
// Zero-alloc random string into a reusable buffer
buf := make([]byte, 32)
fastrand.FillString(buf, fastrand.CharsAlphabetDigits)
fmt.Println(string(buf)) // 32 random alphanumeric chars, 0 allocations

// Zero-alloc hex token
hexBuf := make([]byte, 64)
fastrand.FillHex(hexBuf)
fmt.Println(string(hexBuf)) // 64 hex chars, 0 allocations

// Zero-alloc secure bytes
secureBuf := make([]byte, 256)
err := fastrand.SecureFillBytes(secureBuf) // single mutex lock, 0 allocations
```

### Collections

- `Choice[T any](items []T) T` — pick one random element
- `ChoiceMultiple[T any](items []T, count int) []T` — pick `count` unique elements (partial Fisher-Yates)
- `ChoiceKey[T comparable, V any](items map[T]V) T` — pick a random map key
- `ChoiceItemNullable[T any](slice []T) (*T, error)` — pick one element, return pointer or error on empty
- `Shuffle(n int, swap func(i, j int))` — Fisher-Yates shuffle (inlined, zero-alloc)
- `Perm(n int) []int` — random permutation of [0, n)

```go
users := []string{"alice", "bob", "carol", "dave", "eve"}

winner := fastrand.Choice(users)
shortlist := fastrand.ChoiceMultiple(users, 3)

fastrand.Shuffle(len(users), func(i, j int) {
	users[i], users[j] = users[j], users[i]
})
```

### Network and IDs

- `IPv4() net.IP` — random IPv4 address
- `IPv6() net.IP` — random IPv6 address
- `SecureIPv4() (net.IP, error)` — secure random IPv4
- `SecureIPv6() (net.IP, error)` — secure random IPv6
- `FastUUID() ([]byte, error)` — RFC 4122 v4 UUID (16 bytes)
- `MustFastUUID() []byte` — panics on error
- `SecureUUID() ([]byte, error)` — cryptographically secure UUID
- `MustSecureUUID() []byte` — panics on error

## Randomizer Engine

The randomizer engine processes template strings containing `{RAND;length;keyword}` placeholders and replaces them with random data. Use it for synthetic data generation, fuzz testing payloads, mock API responses, and structured test fixtures.

- **Package-level**: `RandomizerString(string) string`, `Randomizer([]byte) []byte`
- **Custom engine**: `NewEngine(opts...)` returns `*FastEngine` with configurable behavior

### Placeholder Syntax

```text
{RAND;length;keyword}
{RANDOM;length;keyword}   -- optional OM suffix
{RAND}                     -- defaults: length=16, keyword=ABR
```

Examples:
- `{RAND;8;DIGIT}` — 8 random digits
- `{RAND;5-10;ABU}` — 5–10 random uppercase letters
- `{RAND;UUID,EMAIL}` — randomly choose UUID or email
- `{RAND}` — 16 random mixed-case alphanumeric chars

### Keywords

All keywords are **case-insensitive** (`digit`, `DIGIT`, `Digit` are equivalent):

| Keyword | Output | Example |
|---|---|---|
| `ABL` | Lowercase letters | `abcdefgh` |
| `ABU` | Uppercase letters | `ABCDEFGH` |
| `ABR` | Mixed-case letters | `aBcDeFgH` |
| `DIGIT` | Numeric digits | `12345678` |
| `HEX` | Hex string (length × 2 chars) | `a1b2c3d4` |
| `SPACE` | Space characters | `        ` |
| `NULL` | Bytes 0–15 | `\x00\x01\x02...` |
| `UUID` | RFC 4122 v4 UUID | `550e8400-e29b-41d4-a716-446655440000` |
| `IPV4` | IPv4 address | `192.168.1.1` |
| `IPV6` | IPv6 address | `2001:db8::1` |
| `BYTES` | Raw random bytes | (binary) |
| `EMAIL` | Random email from safe providers | `user@gmail.com` |

### Length Specification

- **Fixed**: `{RAND;8;DIGIT}` — exactly 8
- **Range**: `{RAND;5-10;ABU}` — random between 5 and 10
- **Choices**: `{RAND;5,10,15;DIGIT}` — randomly pick from 5, 10, or 15
- **Default**: `{RAND}` or `{RAND;UUID}` — uses engine default (16)
- **Clamped**: lengths outside `[minLength, maxLength]` fall back to default

### Keyword Choices

Separate multiple keywords with commas to randomly pick one:

- `{RAND;UUID,HEX}` — UUID or hex string
- `{RAND;8-12;HEX,ABL}` — 8–12 chars, either hex or lowercase
- `{RAND;IPV4,IPV6}` — IPv4 or IPv6 address

Disabled keywords are filtered out of choices automatically.

### URL/HTML Encoding

The engine supports both input decoding and output encoding:

- **Input encoding**: decode URL-encoded (`%7BRAND%3B8%3BDIGIT%7D`) or HTML-encoded (`&lbrace;RAND;8;DIGIT&rbrace;`) templates before processing
- **Output encoding**: URL-encode (`RandomizerEncodingURL`) or HTML-encode (`RandomizerEncodingHTML`) the non-placeholder portions of output

### Engine Options

```go
engine := fastrand.NewEngine(
	fastrand.WithDefaultLength(12),
	fastrand.WithMinLength(4),
	fastrand.WithMaxLength(64),
	fastrand.WithDisabledKeywords("UUID", "IPV6"),
	fastrand.WithCustomKeyword("ENV", func(length int) []byte {
		return []byte("prod")
	}),
	fastrand.WithCustomCharset("DIGIT", []byte("01")),
	fastrand.WithMailProviders([]string{"example.com", "test.org"}),
	fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL),
	fastrand.WithOutputEncoding(fastrand.RandomizerEncodingHTML),
	fastrand.WithRanges(false),
	fastrand.WithKeywordChoices(false),
	fastrand.WithLengthChoices(false),
)
```

| Option | Description |
|---|---|
| `WithDefaultLength(n)` | Default length when not specified (default: 16) |
| `WithMinLength(n)` | Minimum allowed length (default: 1) |
| `WithMaxLength(n)` | Maximum allowed length (default: 99) |
| `WithDisabledKeywords(kw...)` | Disable specific keywords |
| `WithCustomKeyword(kw, fn)` | Register a custom keyword generator |
| `WithCustomCharset(kw, cs)` | Override a keyword's charset |
| `WithMailProviders(list)` | Override email domain list |
| `WithInputEncoding(enc)` | Decode input as URL/HTML encoded |
| `WithOutputEncoding(enc)` | Encode non-placeholder output |
| `WithRanges(bool)` | Enable/disable length ranges (default: true) |
| `WithKeywordChoices(bool)` | Enable/disable keyword choices (default: true) |
| `WithLengthChoices(bool)` | Enable/disable length choices (default: true) |

### Example: Template Generation

```go
// Package-level
payload := "user={RAND;8;ABL}&pin={RAND;6;DIGIT}&id={RAND;UUID}"
out := fastrand.RandomizerString(payload)
// Output: user=qwertyui&pin=123456&id=550e8400-e29b-41d4-a716-446655440000
```

### Example: Custom Engine

```go
engine := fastrand.NewEngine(
	fastrand.WithDefaultLength(12),
	fastrand.WithMinLength(4),
	fastrand.WithMaxLength(64),
	fastrand.WithCustomKeyword("ENV", func(length int) []byte {
		return []byte("prod")
	}),
)

out := engine.RandomizerString("service={RAND;ENV}&key={RAND;16;HEX}")
// Output: service=prod&key=a1b2c3d4e5f6a7b8
```

### RandomizerAppend — Zero-Allocation Output

`RandomizerAppend` appends randomized output to a caller-provided buffer, achieving **zero allocations** when the buffer has sufficient capacity:

```go
engine := fastrand.NewEngine()

// Pre-allocate a reusable buffer
dst := make([]byte, 0, 512)
result := engine.RandomizerAppend(dst, []byte("user={RAND;8;ABL}&id={RAND;UUID}"))
// result is dst with randomized content appended — 1 allocation total (the buffer)

// Chain multiple appends
dst = engine.RandomizerAppend(dst, []byte("&token={RAND;32;HEX}"))
// dst now contains all three placeholders, still 1 allocation
```

## Concurrency

All package-level functions and `FastEngine` methods are safe for concurrent use across goroutines:

- Fast path uses `atomic.Uint64.Add` — fully lock-free
- Secure path uses `sync.Mutex` around ChaCha8 source
- `FastEngine` is safe to share across goroutines (no mutable state after construction)

```go
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = fastrand.String(32, fastrand.CharsAlphabetDigits)
		_ = fastrand.SecureBytes(64)
	}()
}
wg.Wait()
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=Benchmark -benchmem -run=^$ -benchtime=1s ./...
```

The test suite includes 200+ test cases covering:
- All fill APIs (edge cases, odd lengths, various sizes 1–1025)
- Charset uniformity validation (statistical distribution checks)
- Case-insensitive keyword matching (all 12 keywords × lower/upper/mixed)
- Malformed tag round-trip preservation
- Concurrency stress tests (50 goroutines × 200+ ops)
- IPv4/IPv6/UUID/Email format validation (1000 iterations each)
- Engine option combinations (disabled features, custom keywords, encoding)

## Benchmarks

```
BenchmarkIntnFastRand-24             	162M ns/op    7.2 ns    0 B    0 allocs
BenchmarkFloat64FastRand-24          	343M ns/op    3.5 ns    0 B    0 allocs
BenchmarkFillBytes-24                	24M  ns/op     50 ns    0 B    0 allocs
BenchmarkFillString-24               	4.7M ns/op    256 ns    0 B    0 allocs
BenchmarkFillHex-24                  	24M  ns/op     50 ns    0 B    0 allocs
BenchmarkSecureFillBytes-24          	35M  ns/op     35 ns    0 B    0 allocs
BenchmarkSecureFillString-24         	7.4M ns/op   150 ns    0 B    0 allocs
BenchmarkStringFastRand/Size32-24    	4.4M ns/op   275 ns   32 B   1 alloc
BenchmarkSecureBytesFastRand/Size4096 522K ns/op  2340 ns 4096 B   1 alloc
BenchmarkEngine-24                   	964K ns/op   1219 ns  640 B   1 alloc
BenchmarkRandomizer-24               	868K ns/op   1344 ns  640 B   1 alloc
BenchmarkRandomizerAppend-24         	897K ns/op   1367 ns  512 B   1 alloc
```

## License

MIT. See [LICENSE](LICENSE).