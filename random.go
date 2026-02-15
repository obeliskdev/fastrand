package fastrand

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"math/rand/v2"
	"net"
	"time"
	"unsafe"
)

type CharsList []byte

var (
	CharsNull           = CharsList{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	CharsSymbolChars    = CharsList("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")
	CharsAlphabetLower  = CharsList("abcdefghijklmnopqrstuvwxyz")
	CharsAlphabetUpper  = CharsList("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	CharsDigits         = CharsList("0123456789")
	CharsSpace          = CharsList(" ")
	CharsAlphabet       = append(CharsAlphabetLower, CharsAlphabetUpper...)
	CharsAlphabetDigits = append(CharsAlphabet, CharsDigits...)
	CharsAll            = append(CharsAlphabetDigits, CharsSymbolChars...)
)

type number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

var (
	pcgSrc       *rand.Rand
	chaChaSrc    *rand.Rand
	FastReader   io.Reader
	SecureReader io.Reader
)

func init() {
	var seed1, seed2 uint64
	seedBytes := make([]byte, 16)
	if _, err := crand.Read(seedBytes); err != nil {
		nano := uint64(time.Now().UnixNano())
		seed1 = nano
		seed2 = bits.Reverse64(nano)
	} else {
		seed1 = binary.LittleEndian.Uint64(seedBytes[:8])
		seed2 = binary.LittleEndian.Uint64(seedBytes[8:])
	}
	pcgSource := rand.NewPCG(seed1, seed2)
	pcgSrc = rand.New(pcgSource)

	var chachaSeed [32]byte
	if _, err := crand.Read(chachaSeed[:]); err != nil {
		nano := uint64(time.Now().UnixNano())
		binary.LittleEndian.PutUint64(chachaSeed[0:8], nano)
		binary.LittleEndian.PutUint64(chachaSeed[8:16], bits.Reverse64(nano))
		binary.LittleEndian.PutUint64(chachaSeed[16:24], nano>>5)
		binary.LittleEndian.PutUint64(chachaSeed[24:32], nano<<5)
	}
	chaChaSource := rand.NewChaCha8(chachaSeed)
	chaChaSrc = rand.New(chaChaSource)

	FastReader = &randReader{src: pcgSource}
	SecureReader = &randReader{src: chaChaSource}
}

type randReader struct {
	src rand.Source
}

func (r *randReader) Read(p []byte) (n int, err error) {
	n = len(p)
	read := 0
	for read < n {
		val := r.src.Uint64()
		remaining := n - read
		if remaining >= 8 {
			binary.LittleEndian.PutUint64(p[read:], val)
			read += 8
		} else {
			var tempBuf [8]byte
			binary.LittleEndian.PutUint64(tempBuf[:], val)
			copy(p[read:], tempBuf[:remaining])
			read += remaining
		}
	}
	return n, nil
}

func Int(min, max int) int {
	if min > max {
		panic(fmt.Sprintf("fastrand: invalid integer range [%d, %d]", min, max))
	}
	if min == max {
		return min
	}
	return min + pcgSrc.IntN(max-min+1)
}

func IntN(n int) int {
	if n <= 0 {
		panic("fastrand: argument n must be positive")
	}
	return pcgSrc.IntN(n)
}

func Bytes(length int) []byte {
	if length < 0 {
		panic("fastrand: length cannot be negative")
	}
	if length == 0 {
		return []byte{}
	}
	b := make([]byte, length)
	if _, err := FastReader.Read(b); err != nil {
		panic(fmt.Sprintf("fastrand: failed to read random bytes: %v", err))
	}
	return b
}

func Hex(length int) string {
	return fmt.Sprintf("%x", Bytes(length))
}

func SecureHex(length int) (string, error) {
	bytes, err := SecureBytes(length)
	if err != nil {
		return "", fmt.Errorf("fastrand: failed to generate secure hex: %w", err)
	}
	return fmt.Sprintf("%x", bytes), nil
}

func String(length int, charset CharsList) string {
	if length <= 0 {
		panic("fastrand: length must be positive")
	}

	csLen := len(charset)

	if csLen == 0 {
		panic("fastrand: charset must not be empty")
	}

	b := make([]byte, length)

	for i := 0; i < length; i++ {
		b[i] = charset[IntN(csLen)]
	}

	return *(*string)(unsafe.Pointer(&b))
}

func Choice[T any](items []T) T {
	if len(items) == 0 {
		panic("fastrand: cannot choose from an empty slice")
	}
	return items[IntN(len(items))]
}

func ChoiceKey[T comparable, V any](items map[T]V) T {
	if len(items) == 0 {
		panic("fastrand: cannot choose from an empty map")
	}

	i := IntN(len(items))
	for k := range items {
		if i == 0 {
			return k
		}
		i--
	}

	panic("unreachable")
}

func ChoiceItemNullable[T any](slice []T) (*T, error) {
	if len(slice) == 0 {
		return nil, errors.New("fastrand: cannot choose from an empty slice")
	}
	return &slice[IntN(len(slice))], nil
}

func Bool() bool {
	return IntN(2) == 1
}

func ChoiceMultiple[T any](items []T, count int) []T {
	n := len(items)
	if n == 0 {
		return []T{}
	}

	if count <= 0 || count >= n {
		shuffled := make([]T, n)
		copy(shuffled, items)
		pcgSrc.Shuffle(n, func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		return shuffled
	}

	chosen := make([]T, count)

	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}

	pcgSrc.Shuffle(n, func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	for i := 0; i < count; i++ {
		chosen[i] = items[indices[i]]
	}

	return chosen
}

func IPv4() net.IP {
	return Bytes(net.IPv4len)
}

func IPv6() net.IP {
	return Bytes(net.IPv6len)
}

func Float64() float64 {
	return pcgSrc.Float64()
}

func Byte() byte {
	return byte(pcgSrc.Uint64())
}

func Number[T number](min, max T) T {
	if min > max {
		panic(fmt.Sprintf("fastrand: invalid number range [%v, %v]", min, max))
	}
	if min == max {
		return min
	}
	switch any(min).(type) {
	case float32:
		fmin, fmax := float32(min), float32(max)
		return T(fmin + pcgSrc.Float32()*(fmax-fmin))
	case float64:
		fmin, fmax := float64(min), float64(max)
		return T(fmin + pcgSrc.Float64()*(fmax-fmin))
	case int, int8, int16, int32, int64:
		imin, imax := int64(min), int64(max)
		return T(imin + pcgSrc.Int64N(imax-imin+1))
	case uint, uint8, uint16, uint32, uint64:
		umin, umax := uint64(min), uint64(max)
		return T(umin + pcgSrc.Uint64N(umax-umin+1))
	default:
		panic(fmt.Sprintf("fastrand: unsupported type %T", min))
	}
}

func NumberN[T number](n T) T {
	var zero T
	if n < zero {
		panic(fmt.Sprintf("fastrand: invalid NumberN length %v, must be non-negative", n))
	}
	return Number(zero, n)
}

func Shuffle(n int, swap func(i, j int)) {
	pcgSrc.Shuffle(n, swap)
}

func Perm(n int) []int {
	return pcgSrc.Perm(n)
}

func SecureInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("fastrand: invalid secure integer range [%d, %d]", min, max)
	}
	if min == max {
		return min, nil
	}
	val, err := SecureIntN(max - min + 1)
	if err != nil {
		return 0, err
	}
	return min + val, nil
}

func SecureIntN(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("fastrand: argument n must be positive for SecureIntN")
	}
	return chaChaSrc.IntN(n), nil
}

func SecureBytes(length int) ([]byte, error) {
	if length < 0 {
		return nil, errors.New("fastrand: length cannot be negative")
	}
	if length == 0 {
		return []byte{}, nil
	}
	b := make([]byte, length)
	_, err := SecureReader.Read(b)
	if err != nil {
		return nil, fmt.Errorf("fastrand: failed to generate secure random bytes: %w", err)
	}
	return b, nil
}

func SecureString(length int, charset CharsList) (string, error) {
	if length <= 0 {
		return "", errors.New("fastrand: length must be positive")
	}

	csLen := len(charset)

	if csLen == 0 {
		return "", errors.New("fastrand: charset must not be empty")
	}

	b := make([]byte, length)

	for i := range b {
		idx, err := SecureIntN(csLen)
		if err != nil {
			return "", fmt.Errorf("fastrand: error getting secure index for string: %w", err)
		}
		b[i] = charset[idx]
	}

	return *(*string)(unsafe.Pointer(&b)), nil
}

func SecureIPv4() (net.IP, error) {
	ip := make(net.IP, net.IPv4len)
	_, err := SecureReader.Read(ip)
	if err != nil {
		return nil, fmt.Errorf("fastrand: failed to generate secure IPv4: %w", err)
	}
	return ip, nil
}

func SecureIPv6() (net.IP, error) {
	ip := make(net.IP, net.IPv6len)
	_, err := SecureReader.Read(ip)
	if err != nil {
		return nil, fmt.Errorf("fastrand: failed to generate secure IPv6: %w", err)
	}
	return ip, nil
}

func SecureFloat64() float64 {
	return chaChaSrc.Float64()
}

func SecureByte() byte {
	return byte(chaChaSrc.Uint64())
}

func SecureNumber[T number](min, max T) (T, error) {
	if min > max {
		var zero T
		return zero, fmt.Errorf("fastrand: invalid secure number range [%v, %v]", min, max)
	}
	if min == max {
		return min, nil
	}
	switch any(min).(type) {
	case float32:
		fmin, fmax := float32(min), float32(max)
		return T(fmin + chaChaSrc.Float32()*(fmax-fmin)), nil
	case float64:
		fmin, fmax := float64(min), float64(max)
		return T(fmin + chaChaSrc.Float64()*(fmax-fmin)), nil
	case int, int8, int16, int32, int64:
		imin, imax := int64(min), int64(max)
		randVal := chaChaSrc.Int64N(imax - imin + 1)
		return T(imin + randVal), nil
	case uint, uint8, uint16, uint32, uint64:
		umin, umax := uint64(min), uint64(max)
		randVal := chaChaSrc.Uint64N(umax - umin + 1)
		return T(umin + randVal), nil
	default:
		var zero T
		return zero, fmt.Errorf("fastrand: unsupported type %T", min)
	}
}

func SecureNumberN[T number](n T) (T, error) {
	var zero T
	if n < zero {
		var z T
		return z, fmt.Errorf("fastrand: invalid SecureNumberN length %v, must be non-negative", n)
	}
	return SecureNumber(zero, n)
}

func MustFastUUID() []byte {
	uuid, err := FastUUID()
	if err != nil {
		panic(err)
	}
	return uuid
}

func FastUUID() ([]byte, error) {
	var uuid [16]byte
	if _, err := FastReader.Read(uuid[:]); err != nil {
		return nil, err
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return uuid[:], nil
}

func MustSecureUUID() []byte {
	uuid, err := SecureUUID()
	if err != nil {
		panic(err)
	}
	return uuid
}

func SecureUUID() ([]byte, error) {
	var uuid [16]byte
	if _, err := SecureReader.Read(uuid[:]); err != nil {
		return nil, err
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return uuid[:], nil
}
