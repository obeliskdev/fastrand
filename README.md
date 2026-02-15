# FastRand for Go

[![Go Report Card](https://goreportcard.com/badge/github.com/obeliskdev/fastrand)](https://goreportcard.com/report/github.com/obeliskdev/fastrand)
[![GoDoc](https://godoc.org/github.com/obeliskdev/fastrand?status.svg)](https://godoc.org/github.com/obeliskdev/fastrand)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**FastRand** is a high-performance, zero-dependency Go library for generating random data. It provides a comprehensive suite of tools, from simple numbers to complex templated strings, with both cryptographic (ChaCha8) and non-cryptographic (PCG) sources. It is engineered to be a faster, more powerful, and more ergonomic replacement for Go's standard `math/rand` and `crypto/rand` packages for common tasks.

---

## Table of Contents

1.  [Why FastRand? The Performance Advantage](#why-fastrand-the-performance-advantage)
2.  [Core Features](#core-features)
3.  [Performance Snapshot](#performance-snapshot)
4.  [Installation](#installation)
5.  [Core Concepts: Package vs. Engine](#core-concepts-package-vs-engine)
6.  [**Complete API Reference**](#complete-api-reference)
    *   [Numeric Generation](#numeric-generation)
    *   [String & Byte Generation](#string--byte-generation)
    *   [Slice & Map Utilities](#slice--map-utilities)
    *   [Network & ID Generation](#network--id-generation)
    *   [Cryptographically Secure Functions](#cryptographically-secure-functions)
7.  [**The `Randomizer` Engine: In-Depth**](#the-randomizer-engine-in-depth)
    *   [Placeholder Syntax](#placeholder-syntax)
    *   [Built-in Keywords](#built-in-keywords)
    *   [Dynamic Generation: Ranges and Choices](#dynamic-generation-ranges-and-choices)
8.  [**The Configurable Engine: Full Customization**](#the-configurable-engine-full-customization)
    *   [Creating a Custom Engine](#creating-a-custom-engine)
    *   [Complete List of Engine Options](#complete-list-of-engine-options)
9.  [Concurrency](#concurrency)
10. [Project Information](#project-information)
    *   [Recommended Description](#recommended-description)
    *   [Keywords](#keywords)
11. [License](#license)

---

## Why FastRand? The Performance Advantage

FastRand is not just another random library; it's an intentional upgrade designed to address the performance and ergonomic limitations of Go's standard libraries for common, high-throughput use cases.

#### 1. Superior PRNG Algorithm (vs. `math/rand`)

The default non-secure generator in FastRand is built on Go 1.22+'s `math/rand/v2` with a **PCG (Permuted Congruential Generator)** source.

*   **Speed**: PCG is significantly faster than the legacy `math/rand` source. This allows for primitive generation (ints, floats) in **1-2 nanoseconds**.
*   **Statistical Quality**: PCG offers better statistical properties, passing modern test suites where the old default generator would fail.
*   **Memory Efficiency**: Core functions are engineered to be **zero-allocation**, meaning they can be called millions of times without creating any work for Go's garbage collector. This is critical for maintaining low latency in hot paths.

> **Result:** A generator that is faster, statistically better, and more memory-efficient than the standard library's `math/rand`.

#### 2. High-Performance CSPRNG (vs. `crypto/rand`)

The secure generator is built on **ChaCha8**, a modern, high-speed stream cipher widely used in protocols like TLS 1.3.

*   **In-Process Speed**: `crypto/rand` is a thin wrapper around OS-level entropy sources (e.g., `/dev/urandom`). While secure, this can involve system calls and potential lock contention under high concurrency. ChaCha8 runs entirely within your Go process, making it an extremely fast and reliable source for cryptographic needs.
*   **Ergonomic API**: FastRand provides a rich, developer-friendly API (`SecureIntN`, `SecureString`, `SecureNumber`) that is much more convenient than manually reading from `crypto.Reader` and performing biasing calculations.

> **Result:** You get the security guarantees needed for cryptographic tasks with a significant performance boost and a much better developer experience.

#### 3. Extreme Memory Optimization

The `Randomizer` engine is the library's most powerful feature, but it's also a performance showcase.

*   **Buffer Pooling**: Instead of creating and discarding `bytes.Buffer` objects on every call, the engine uses a **`bytebufferpool`** to recycle memory. This dramatically reduces GC pressure in applications that generate many templated strings.
*   **Allocation-Free Parsing**: The engine's internal parser is handwritten to avoid Go's standard `strings.Split` function, which allocates new slices. By manually scanning for delimiters, the engine processes templates with almost no memory overhead.

> **Result:** A templating engine that is not only powerful but also incredibly efficient, making it suitable for performance-critical tasks like generating mock logs, API responses, or test data at scale.

## Core Features

-   **Dual Random Sources**: Blazing-fast PCG for general use and secure ChaCha8 for cryptographic needs.
-   **Configurable `Randomizer` Engine**: Create isolated engine instances with custom rules, keywords, character sets, and length constraints.
-   **Simple & Idiomatic API**: Intuitive functions like `IntN`, `String`, and `Bytes`.
-   **Type-Safe Generics**: Generate random numbers for any standard integer or float type with `Number[T]()`.
-   **Rich Helper Functions**: One-line functions for `IPv4`, `IPv6`, `UUID`, `Hex`, and more.
-   **Powerful Slice & Map Tools**: `Choice` an element, select `ChoiceMultiple` elements, `Shuffle` a slice, or pick a `ChoiceKey` from a map.
-   **Concurrency Safe**: All functions are safe for concurrent use in goroutines.
-   **Zero Dependencies**: Relies only on the Go standard library and the optional, high-performance `bytebufferpool`.

## Performance Snapshot

Benchmarks highlight the significant performance gains over standard practices.

| Benchmark | Speed (`ns/op`) | Memory (`B/op`) | Allocations (`allocs/op`) |
| :--- | :--- | :--- | :--- |
| **`fastrand.IntN`** | `~2.4 ns/op` | `0 B/op` | `0 allocs/op` |
| **`fastrand.SecureBytes(64)`** | `~42 ns/op` | `64 B/op` | `1 alloc/op` |
| **`crypto/rand.Read(64)`** | `~76 ns/op` | `0 B/op` | `0 allocs/op` |
| **`fastrand.Randomizer`** | `~1245 ns/op` | `~693 B/op` | `~13 allocs/op` |

The `Randomizer`'s result of **~13 allocations** for parsing a complex template with 5 different placeholders is exceptionally low and demonstrates the effectiveness of the memory optimizations.

## Installation

```sh
go get github.com/obeliskdev/fastrand
```

## Core Concepts: Package vs. Engine

FastRand offers two modes of operation:

1.  **Package-Level Functions**: For quick and common tasks, you can use functions directly from the package (e.g., `fastrand.IntN(10)`). These all use a pre-configured, shared "default engine" that is optimized for general use.

2.  **The Configurable `Engine`**: For advanced control, you can create your own `Engine` instance (`fastrand.NewEngine(...)`). This gives you an isolated, reusable generator with its own unique set of rules, custom keywords, and output formats. This is the recommended approach for building complex, testable systems.

## Complete API Reference

### Numeric Generation

#### `Int(min, max int) int`
Generates a random integer within the inclusive range `[min, max]`. Panics if `min > max`.
```go
num := fastrand.Int(-50, 50) // e.g., -23
```
*Note: Uses the fast PCG source.*

#### `IntN(n int) int`
Generates a random integer within the half-open range `[0, n)`. Panics if `n <= 0`.
```go
index := fastrand.IntN(100) // e.g., 76
```
*Note: Uses the fast PCG source. Zero-allocation.*

#### `Float64() float64`
Generates a random `float64` in the half-open range `[0.0, 1.0)`.
```go
f := fastrand.Float64() // e.g., 0.12345
```
*Note: Uses the fast PCG source. Zero-allocation.*

#### `Bool() bool`
Returns `true` or `false` with equal probability.
```go
isEnabled := fastrand.Bool() // e.g., true
```

#### `Number[T number](min, max T) T`
A generic function to generate a random number of any standard integer or float type `T` within the inclusive range `[min, max]`.
```go
var num_i16 int16 = fastrand.Number[int16](-1000, 1000)
var num_u32 uint32 = fastrand.Number[uint32](0, 50000)
var num_f32 float32 = fastrand.Number[float32](-10.5, 10.5)
```

#### `NumberN[T number](n T) T`
A generic function to generate a random number of type `T` within the inclusive range `[0, n]`.
```go
val := fastrand.NumberN[int64](10000)
```

### String & Byte Generation

#### `Bytes(length int) []byte`
Generates a slice of `length` random bytes.
```go
data := fastrand.Bytes(16)
```
*Note: Uses the fast PCG source. Performs one allocation for the slice.*

#### `Hex(length int) string`
Generates `length` random bytes and returns them as a 2x-length hexadecimal string.
```go
token := fastrand.Hex(8) // e.g., "a1b2c3d4e5f6a7b8" (16 chars)
```

#### `String(length int, charset CharsList) string`
Generates a random string of a given `length` using characters from the provided `charset`.
```go
// Pre-defined charsets:
// CharsDigits, CharsAlphabetLower, CharsAlphabetUpper, CharsAlphabet,
// CharsAlphabetDigits, CharsSymbolChars, CharsAll

pin := fastrand.String(8, fastrand.CharsDigits) // e.g., "91827364"
id := fastrand.String(12, fastrand.CharsAlphabetUpper) // e.g., "QWERTYASDFZX"
```

### Slice & Map Utilities

#### `Choice[T any](items []T) T`
Selects and returns one random element from a slice. Panics if the slice is empty.
```go
names := []string{"Alice", "Bob", "Charlie"}
winner := fastrand.Choice(names) // e.g., "Bob"
```

#### `ChoiceKey[T comparable, V any](items map[T]V) T`
Selects and returns one random key from a map. Panics if the map is empty.
```go
scores := map[string]int{"alpha": 100, "beta": 200}
team := fastrand.ChoiceKey(scores) // e.g., "beta"
```

#### `ChoiceMultiple[T any](items []T, count int) []T`
Selects `count` unique random elements from a slice and returns them in a new slice.
```go
users := []string{"A", "B", "C", "D", "E"}
winners := fastrand.ChoiceMultiple(users, 3) // e.g., ["D", "A", "C"]
```
*Note: Uses an efficient partial shuffle algorithm.*

#### `Shuffle(n int, swap func(i, j int))`
Shuffles a collection of `n` elements in place using the provided `swap` function.
```go
numbers := []int{1, 2, 3, 4, 5}
fastrand.Shuffle(len(numbers), func(i, j int) {
	numbers[i], numbers[j] = numbers[j], numbers[i]
})
// numbers is now shuffled, e.g., [3, 1, 5, 2, 4]
```

#### `Perm(n int) []int`
Returns a random permutation of the integers `[0, n)`.
```go
indices := fastrand.Perm(5) // e.g., [4, 1, 0, 3, 2]
```

### Network & ID Generation

#### `IPv4() net.IP`
Generates a random `net.IP` representing a v4 address.
```go
ip := fastrand.IPv4() // e.g., 198.51.100.10
```

#### `IPv6() net.IP`
Generates a random `net.IP` representing a v6 address.
```go
ip := fastrand.IPv6() // e.g., 2001:db8::1234:5678
```

#### `MustFastUUID() []byte`
Generates a fast, non-secure v4 UUID as a 16-byte slice. Panics on error.
```go
uuid := fastrand.MustFastUUID()
```

### Cryptographically Secure Functions

These functions use the **ChaCha8** source and are suitable for generating passwords, keys, tokens, and other sensitive data. All `Secure*` functions that can fail return an `error`.

-   **`SecureInt(min, max int) (int, error)`**: Secure integer in `[min, max]`.
-   **`SecureBytes(length int) ([]byte, error)`**: Secure byte slice.
-   **`SecureHex(length int) (string, error)`**: Secure hex string.
-   **`SecureString(length int, charset CharsList) (string, error)`**: Secure string from a charset.
-   **`SecureIPv4() (net.IP, error)`**: Secure IPv4 address.
-   **`SecureIPv6() (net.IP, error)`**: Secure IPv6 address.
-   **`SecureFloat64() float64`**: Secure float in `[0.0, 1.0)`.
-   **`SecureNumber[T number](min, max T) (T, error)`**: Secure generic number.
-   **`MustSecureUUID() []byte`**: Secure v4 UUID. Panics on error.

```go
// Example: Generate a 32-character secure password
password, err := fastrand.SecureString(32, fastrand.CharsAll)
if err != nil {
    panic(err)
}
fmt.Println("Secure Password:", password)
```

---

## The `Randomizer` Engine: In-Depth

The `Randomizer` is the library's most powerful feature, allowing you to generate complex, structured data from a single template string.

### Placeholder Syntax

The engine parses a string and replaces placeholders with random data. The syntax is flexible and powerful:

`{RAND[OM];[LENGTH];[TYPE]}`

-   **`OM`**: The `OM` is optional. `{RAND...}` and `{RANDOM...}` are equivalent.
-   **`[LENGTH]`**: An optional parameter that can be:
    *   A single integer: `{RAND;10;...}`
    *   A comma-separated list of choices: `{RAND;5,10,15;...}`
    *   A hyphen-separated range: `{RAND;5-10;...}`
-   **`[TYPE]`**: An optional keyword specifying the data type. Can be a single keyword or a comma-separated list of choices.

### Built-in Keywords

| Keyword | Description | Example Output (for length 8) |
| :--- | :--- | :--- |
| **`ABL`** | Alphabet, Lowercase | `abcdefgh` |
| **`ABU`** | Alphabet, Uppercase | `ABCDEFGH` |
| **`ABR`** | Alphabet, Random Case | `AbCdEfGh` |
| **`DIGIT`** | Digits (`0`-`9`) | `12345678` |
| **`HEX`** | Hexadecimal (`0`-`f`) | `a1b2c3d4e5f6a7b8` (16 chars) |
| **`UUID`** | A v4 UUID (length ignored) | `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx` |
| **`IPV4`** | An IPv4 address (length ignored) | `192.0.2.1` |
| **`IPV6`** | An IPv6 address (length ignored) | `2001:db8::...` |
| **`EMAIL`** | A random email address | `abcdefgh@gmail.com` |
| **`BYTES`** | Raw bytes (length respected) | `[...8 bytes...]` |
| **`SPACE`** | Whitespace characters | ` ` |
| **`NULL`** | Null bytes (`\x00` - `\x0F`) | `[...8 null bytes...]` |
| _(default)_ | `CharsAll` if no keyword | `aB1!c@2#` |

### Dynamic Generation: Ranges and Choices

You can combine these features for maximum flexibility.

```go
// Example: Generates a numeric string that is either 4, 8, or 12 characters long.
id := fastrand.RandomizerString("{RAND;4,8,12;DIGIT}")

// Example: Generates a token that is 10 to 20 characters long and is either hex or uppercase.
token := fastrand.RandomizerString("{RAND;10-20;HEX,ABU}")

// Example: Generates a contact identifier that is either a UUID or an email address.
contact := fastrand.RandomizerString("{RAND;UUID,EMAIL}")
```

---

## The Configurable Engine: Full Customization

While the package-level functions are convenient, the real power of FastRand lies in creating and configuring your own `Engine` instances. This allows you to define isolated, reusable, and testable rule sets for data generation.

### Creating a Custom Engine

The `fastrand.NewEngine(...)` function accepts a series of `Option` functions to build a custom configuration.

```go
// A comprehensive example of a custom engine.
customEngine := fastrand.NewEngine(
    // --- Length Constraints ---
    fastrand.WithDefaultLength(20),
    fastrand.WithMinLength(10),
    fastrand.WithMaxLength(100),

    // --- Feature Toggles ---
    fastrand.WithRanges(false),         // Disable "5-10" syntax
    fastrand.WithKeywordChoices(true),  // Keep "HEX,UUID" syntax enabled

    // --- Encoding Rules ---
    fastrand.WithInputEncoding(fastrand.RandomizerEncodingURL), // ONLY accept URL-encoded tags
    fastrand.WithOutputEncoding(fastrand.RandomizerEncodingHTML), // HTML-escape literal output

    // --- Keyword & Charset Customization ---
    fastrand.WithDisabledKeywords("IPV6", "SPACE"), // Turn off specific keywords
    fastrand.WithCustomCharset("DIGIT", []byte("01")), // Make {RAND;..;DIGIT} produce binary
    fastrand.WithCustomKeyword("PRODUCT_ID", func(length int) []byte {
        return []byte("PROD-" + fastrand.String(length, fastrand.CharsAlphabetUpper))
    }),
)

// Now, use the highly customized engine:
template := "<item pid=\"{RAND;12;PRODUCT_ID}\" code=\"{RAND;16;DIGIT}\" />"
output := customEngine.RandomizerString(template)

// Possible Output:
// &lt;item pid=&#34;PROD-QWERASDFZXCV&#34; code=&#34;0110101100101101&#34; /&gt;
```

### Complete List of Engine Options

| Option Function | Description | Default |
| :--- | :--- | :--- |
| `WithDefaultLength(int)` | Sets the fallback length if none is provided. | `16` |
| `WithMinLength(int)` | Enforces a minimum length for generated data. | `1` |
| `WithMaxLength(int)` | Enforces a maximum length for generated data. | `99` |
| `WithRanges(bool)` | Enables/disables parsing of length ranges (`5-10`). | `true` |
| `WithLengthChoices(bool)` | Enables/disables parsing of length choices (`5,10`). | `true` |
| `WithKeywordChoices(bool)` | Enables/disables parsing of keyword choices (`HEX,UUID`). | `true` |
| `WithInputEncoding(RandomizerEncoding)` | Bitmask for recognized input encodings. | `URL \| HTML` |
| `WithOutputEncoding(RandomizerEncoding)` | Sets encoding for literal output text. | `None` |
| `WithDisabledKeywords(...string)` | Disables one or more built-in keywords. | (none) |
| `WithMailProviders([]string)` | Sets a custom list of email domains. | (embedded list) |
| `WithCustomCharset(string, []byte)` | Overrides the character set for a keyword. | (none) |
| `WithCustomKeyword(string, func)` | Defines a new custom keyword. | (none) |

---

## Concurrency

**This library is fully concurrency-safe.**

Both the PCG and ChaCha8 random sources provided by `math/rand/v2` are designed for safe concurrent use across multiple goroutines. You can safely call any package-level function or use an `Engine` instance from multiple goroutines without needing external locks.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. see the [LICENSE](LICENSE) file for details.