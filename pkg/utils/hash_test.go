package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSha256(t *testing.T) {
	tests := []struct {
		provided string
		expected string
	}{
		{
			provided: "hello",
			expected: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			provided: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed at tellus id sapien auctor fermentum eu a felis.",
			expected: "72719066dfb65a1951bcc028f779245ecb60cfd64e69d4a1e829ea8d747cbe5f",
		},
		{
			provided: "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, test := range tests {
		result := Sha256(test.provided)
		assert.Equal(t, test.expected, result)
	}
}
