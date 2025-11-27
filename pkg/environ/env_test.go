package environ

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

type envTest[K comparable] struct {
	name     string
	fallback K
	set      *string
	expected K
}

func testEnvGet[K comparable](t *testing.T, tests []envTest[K], fn func(string, K) K) {
	for _, test := range tests {
		if test.set != nil {
			err := os.Setenv(test.name, *test.set)
			assert.NoError(t, err)
		}
		assert.Equal(t, test.expected, fn(test.name, test.fallback))
	}
}

func TestGetBool(t *testing.T) {
	tests := []envTest[bool]{
		{
			name:     "bool1",
			fallback: true,
			set:      nil,
			expected: true,
		},
		{
			name:     "bool2",
			fallback: true,
			set:      ptr.To("false"),
			expected: false,
		},
		{
			name:     "bool3",
			fallback: false,
			set:      ptr.To("true"),
			expected: true,
		},
	}
	testEnvGet(t, tests, GetBool)
}

func TestGetDuration(t *testing.T) {
	tests := []envTest[time.Duration]{
		{
			name:     "duration1",
			fallback: time.Minute,
			set:      nil,
			expected: time.Minute,
		},
		{
			name:     "duration2",
			fallback: time.Minute,
			set:      ptr.To("1d"),
			expected: 24 * time.Hour,
		},
	}
	testEnvGet(t, tests, GetDuration)
}

func TestGetInt(t *testing.T) {
	tests := []envTest[int]{
		{
			name:     "int1",
			fallback: 10,
			set:      nil,
			expected: 10,
		},
		{
			name:     "int2",
			fallback: 0,
			set:      ptr.To("10"),
			expected: 10,
		},
	}
	testEnvGet(t, tests, GetInt)
}

func TestGetInt64(t *testing.T) {
	tests := []envTest[int64]{
		{
			name:     "int64_1",
			fallback: 10,
			set:      nil,
			expected: 10,
		},
		{
			name:     "int64_2",
			fallback: 0,
			set:      ptr.To("10"),
			expected: 10,
		},
	}
	testEnvGet(t, tests, GetInt64)
}

func TestGetUint64(t *testing.T) {
	tests := []envTest[uint64]{
		{
			name:     "uint64_1",
			fallback: 10,
			set:      nil,
			expected: 10,
		},
		{
			name:     "uint64_2",
			fallback: 0,
			set:      ptr.To("10"),
			expected: 10,
		},
	}
	testEnvGet(t, tests, GetUint64)
}

func TestGetString(t *testing.T) {
	tests := []envTest[string]{
		{
			name:     "string1",
			fallback: "hello",
			set:      nil,
			expected: "hello",
		},
		{
			name:     "string2",
			fallback: "hello",
			set:      ptr.To("world"),
			expected: "world",
		},
	}
	testEnvGet(t, tests, GetString)
}
