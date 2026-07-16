package fastrand_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/obeliskdev/fastrand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mpEmailUserRegex   = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@`)
	mpEmailDomainRegex = regexp.MustCompile(`@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

func TestWithMailProviders_SetsProviders(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("custom.example", "another.test"),
	)

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		result := engine.RandomizerString("{RAND;8;EMAIL}")
		emailStr := strings.TrimPrefix(result, "")
		assert.True(t, mpEmailUserRegex.MatchString(emailStr), "email should have valid user part: %q", emailStr)

		parts := strings.SplitN(emailStr, "@", 2)
		require.Len(t, parts, 2, "email should contain @: %q", emailStr)
		domain := parts[1]
		assert.True(t, domain == "custom.example" || domain == "another.test",
			"domain %q should be one of the custom providers", domain)
		seen[domain] = true
	}
	assert.True(t, seen["custom.example"], "custom.example should have appeared")
	assert.True(t, seen["another.test"], "another.test should have appeared")
}

func TestWithMailProviders_EmptyArgs_Ignored(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders(),
	)
	assert.Equal(t, fastrand.SafeMailProviders, engine.MailProviders(),
		"no args should keep default providers")
}

func TestWithMailProviders_AllEmptyStrings_Ignored(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("", "", ""),
	)
	assert.Equal(t, fastrand.SafeMailProviders, engine.MailProviders(),
		"all empty strings should keep default providers")
}

func TestWithMailProviders_FiltersEmptyStrings(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("valid.com", "", "also-valid.org", ""),
	)
	providers := engine.MailProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "valid.com")
	assert.Contains(t, providers, "also-valid.org")
}

func TestWithMailProviders_SingleProvider(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("only.domain.com"),
	)
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;8;EMAIL}")
		parts := strings.SplitN(result, "@", 2)
		require.Len(t, parts, 2)
		assert.Equal(t, "only.domain.com", parts[1],
			"single provider should always be used")
	}
}

func TestWithMailProviders_EmailFormat(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("test.io", "mail.dev"),
	)
	for i := 0; i < 500; i++ {
		result := engine.RandomizerString("Contact: {RAND;12;EMAIL}")
		emailStr := strings.TrimPrefix(result, "Contact: ")
		assert.True(t, mpEmailUserRegex.MatchString(emailStr),
			"user part invalid: %q", emailStr)
		assert.True(t, mpEmailDomainRegex.MatchString(emailStr),
			"domain part invalid: %q", emailStr)
	}
}

func TestWithMailProviders_ResetRestoresDefaults(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("temp.com"),
	)
	assert.Contains(t, engine.MailProviders(), "temp.com")

	engine.Reset()
	assert.Equal(t, fastrand.SafeMailProviders, engine.MailProviders(),
		"Reset should restore default providers")
}

func TestWithMailProviders_DoesNotMutateCallerSlice(t *testing.T) {
	providers := []string{"a.com", "b.com", "c.com"}
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders(providers...),
	)
	_ = engine.RandomizerString("{RAND;8;EMAIL}")

	providers[0] = "mutated.com"
	got := engine.MailProviders()
	assert.NotContains(t, got, "mutated.com",
		"caller slice mutation should not affect engine")
}

func TestWithMailProviders_CustomUserLength(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("example.com"),
	)
	for i := 0; i < 100; i++ {
		result := engine.RandomizerString("{RAND;20;EMAIL}")
		parts := strings.SplitN(result, "@", 2)
		require.Len(t, parts, 2)
		assert.Len(t, parts[0], 20, "user part should be exactly 20 chars")
		assert.Equal(t, "example.com", parts[1])
	}
}

func TestWithMailProviders_DefaultUserLength(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("example.com"),
	)
	result := engine.RandomizerString("{RAND;EMAIL}")
	parts := strings.SplitN(result, "@", 2)
	require.Len(t, parts, 2)
	assert.Len(t, parts[0], 16, "default engine length should be 16 (defaultLength)")
}

func TestWithMailProviders_AllDefaultProvidersValid(t *testing.T) {
	for _, provider := range fastrand.SafeMailProviders {
		assert.True(t, mpEmailDomainRegex.MatchString("user@"+provider),
			"SafeMailProvider %q should produce valid domain", provider)
	}
}

func TestWithMailProviders_NoAllocOnSet(t *testing.T) {
	providers := []string{"x.com", "y.com", "z.com"}

	allocs := testing.AllocsPerRun(100, func() {
		_ = fastrand.NewEngine(fastrand.WithMailProviders(providers...))
	})

	if allocs > 15 {
		t.Errorf("NewEngine with WithMailProviders allocated %v times, expected <= 15", allocs)
	}
}

func TestWithMailProviders_EmailBytes(t *testing.T) {
	engine := fastrand.NewEngine(
		fastrand.WithMailProviders("bytes.test"),
	)
	payload := []byte("Email: {RAND;8;EMAIL}")
	result := engine.Randomizer(payload)
	assert.True(t, bytes.HasPrefix(result, []byte("Email: ")), "prefix should be preserved")
	emailPart := result[7:]
	assert.True(t, bytes.Contains(emailPart, []byte("@bytes.test")),
		"email should contain @bytes.test: %q", emailPart)
}
