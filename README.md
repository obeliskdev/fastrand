# FastRand

FastRand is a high-performance random data library for Go.

It provides:
- Fast non-cryptographic randomness for simulations, IDs, shuffling, and sampling.
- Secure randomness for tokens, passwords, and sensitive values.
- A template-driven randomizer engine for generating structured synthetic data.

## Why use FastRand

- One package for both speed-focused and security-focused random generation.
- Simple package-level helpers for common tasks.
- Configurable engine API when you need deterministic formatting rules.
- Generic numeric helpers (`Number[T]`, `SecureNumber[T]`) across integer and float types.

## Installation

```bash
go get github.com/obeliskdev/fastrand
```

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/obeliskdev/fastrand"
)

func main() {
	fmt.Println("IntN:", fastrand.IntN(100))
	fmt.Println("Bool:", fastrand.Bool())
	fmt.Println("Hex token:", fastrand.Hex(16))

	pwd, err := fastrand.SecureString(24, fastrand.CharsAll)
	if err != nil {
		panic(err)
	}
	fmt.Println("Secure password:", pwd)
}
```

## Core API

### Numeric

- `Int(min, max int) int`
- `IntN(n int) int`
- `Float64() float64`
- `Number[T number](min, max T) T`
- `NumberN[T number](n T) T`

### Secure numeric

- `SecureInt(min, max int) (int, error)`
- `SecureIntN(n int) (int, error)`
- `SecureFloat64() float64`
- `SecureNumber[T number](min, max T) (T, error)`
- `SecureNumberN[T number](n T) (T, error)`

### Bytes and strings

- `Bytes(length int) []byte`
- `String(length int, charset CharsList) string`
- `Hex(length int) string`
- `SecureBytes(length int) ([]byte, error)`
- `SecureString(length int, charset CharsList) (string, error)`
- `SecureHex(length int) (string, error)`

### Collections

- `Choice[T any](items []T) T`
- `ChoiceMultiple[T any](items []T, count int) []T`
- `ChoiceKey[T comparable, V any](items map[T]V) T`
- `Shuffle(n int, swap func(i, j int))`
- `Perm(n int) []int`

### Network and IDs

- `IPv4() net.IP`, `IPv6() net.IP`
- `SecureIPv4() (net.IP, error)`, `SecureIPv6() (net.IP, error)`
- `FastUUID() ([]byte, error)`, `MustFastUUID() []byte`
- `SecureUUID() ([]byte, error)`, `MustSecureUUID() []byte`

## Example: Sampling and Shuffling

```go
users := []string{"alice", "bob", "carol", "dave", "eve"}

winner := fastrand.Choice(users)
shortlist := fastrand.ChoiceMultiple(users, 3)

fastrand.Shuffle(len(users), func(i, j int) {
	users[i], users[j] = users[j], users[i]
})
```

## Randomizer Engine

Use templates when you need structured random output.

- Package-level helpers: `RandomizerString(...)`, `Randomizer(...)`
- Custom engine: `NewEngine(opts...)`

Basic placeholder format:

```text
{RAND;length;keyword}
```

Examples:
- `{RAND;8;DIGIT}`
- `{RAND;5-10;ABU}`
- `{RAND;UUID,EMAIL}`

### Example: Template Generation

```go
payload := "user={RAND;8;ABL}&pin={RAND;6;DIGIT}&id={RAND;UUID}"
out := fastrand.RandomizerString(payload)
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
```

## Concurrency

Package-level functions and engine usage are safe for concurrent use across goroutines.

## Testing

```bash
go test ./...
```

## License

MIT. See `LICENSE`.
