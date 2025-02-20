package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTomlEncode(t *testing.T) {
	tests := []struct {
		provided interface{}
		expected string
	}{
		{
			provided: map[string]interface{}{
				"rpc": map[string]interface{}{
					"enabled": true,
					"laddr":   "tcp://127.0.0.1:26657",
				},
			},
			expected: "[rpc]\n  enabled = true\n  laddr = \"tcp://127.0.0.1:26657\"\n",
		},
	}

	for _, test := range tests {
		result, err := TomlEncode(test.provided)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result)
	}
}

func TestTomlDecode(t *testing.T) {
	tests := []struct {
		provided string
		expected interface{}
	}{
		{
			provided: "[rpc]\n  enabled = true\n  laddr = \"tcp://127.0.0.1:26657\"\n",
			expected: map[string]interface{}{
				"rpc": map[string]interface{}{
					"enabled": true,
					"laddr":   "tcp://127.0.0.1:26657",
				},
			},
		},
	}

	for _, test := range tests {
		result, err := TomlDecode(test.provided)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result)
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		provided interface{}
		patch    interface{}
		expected interface{}
	}{
		{
			provided: map[string]interface{}{
				"rpc": map[string]interface{}{
					"enabled": false,
					"laddr":   "tcp://127.0.0.1:26657",
				},
			},
			patch: map[string]interface{}{
				"rpc": map[string]interface{}{
					"enabled": true,
					"laddr":   "tcp://0.0.0.0:26657",
				},
			},
			expected: map[string]interface{}{
				"rpc": map[string]interface{}{
					"enabled": true,
					"laddr":   "tcp://0.0.0.0:26657",
				},
			},
		},
	}

	for _, test := range tests {
		result, err := Merge(test.provided, test.patch)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, result)
	}
}
