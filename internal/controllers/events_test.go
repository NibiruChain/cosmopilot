package controllers

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFormatEventMessage(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "simple message",
			format:   "test message",
			args:     nil,
			expected: "test message",
		},
		{
			name:     "message with formatting",
			format:   "error: %s",
			args:     []interface{}{"something went wrong"},
			expected: "error: something went wrong",
		},
		{
			name:     "message with newlines",
			format:   "line1\nline2\nline3",
			args:     nil,
			expected: "line1 line2 line3",
		},
		{
			name:     "message with tabs and carriage returns",
			format:   "col1\tcol2\rcol3",
			args:     nil,
			expected: "col1 col2 col3",
		},
		{
			name:     "message with multiple spaces",
			format:   "too    many     spaces",
			args:     nil,
			expected: "too many spaces",
		},
		{
			name:     "message with leading/trailing whitespace",
			format:   "  trim me  ",
			args:     nil,
			expected: "trim me",
		},
		{
			name:     "long message gets truncated",
			format:   strings.Repeat("a", 300),
			args:     nil,
			expected: strings.Repeat("a", maxEventMessageLength-3) + "...",
		},
		{
			name:     "message exactly at limit",
			format:   strings.Repeat("a", maxEventMessageLength),
			args:     nil,
			expected: strings.Repeat("a", maxEventMessageLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEventMessage(tt.format, tt.args...)
			if result != tt.expected {
				t.Errorf("FormatEventMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatErrorEvent(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			context:  "something happened",
			err:      nil,
			expected: "something happened",
		},
		{
			name:     "simple error",
			context:  "Failed to connect",
			err:      errors.New("connection refused"),
			expected: "Failed to connect: connection refused",
		},
		{
			name:     "wrapped error chain",
			context:  "Failed operation",
			err:      fmt.Errorf("outer: middle: inner: root cause"),
			expected: "Failed operation: root cause",
		},
		{
			name:     "error with verbose prefixes",
			context:  "Something failed",
			err:      errors.New("error failed to complete"),
			expected: "Something failed: complete",
		},
		{
			name:     "error with newlines",
			context:  "Operation failed",
			err:      errors.New("error:\nline1\nline2"),
			expected: "Operation failed: error: line1 line2",
		},
		{
			name:     "very long error message gets truncated",
			context:  "Failed",
			err:      errors.New(strings.Repeat("x", 300)),
			expected: "Failed: " + strings.Repeat("x", maxEventMessageLength-len("Failed: ")-3) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorEvent(tt.context, tt.err)
			if result != tt.expected {
				t.Errorf("FormatErrorEvent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSummarizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "unknown error",
		},
		{
			name:     "timeout error",
			err:      errors.New("operation timeout exceeded"),
			expected: "timeout",
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: "connection refused",
		},
		{
			name:     "host not found",
			err:      errors.New("lookup failed: no such host"),
			expected: "host not found",
		},
		{
			name:     "not found",
			err:      errors.New("resource not found"),
			expected: "not found",
		},
		{
			name:     "already exists",
			err:      errors.New("object already exists"),
			expected: "already exists",
		},
		{
			name:     "forbidden",
			err:      errors.New("access forbidden"),
			expected: "forbidden",
		},
		{
			name:     "unauthorized",
			err:      errors.New("unauthorized access"),
			expected: "unauthorized",
		},
		{
			name:     "invalid",
			err:      errors.New("invalid configuration"),
			expected: "invalid",
		},
		{
			name:     "context canceled",
			err:      errors.New("operation failed: context canceled"),
			expected: "canceled",
		},
		{
			name:     "deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: "deadline exceeded",
		},
		{
			name:     "short error with sentence",
			err:      errors.New("failed to start. please retry"),
			expected: "failed to start",
		},
		{
			name:     "long error gets truncated",
			err:      errors.New(strings.Repeat("a", 100)),
			expected: strings.Repeat("a", 47) + "...",
		},
		{
			name:     "short error returned as-is",
			err:      errors.New("short error"),
			expected: "short error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SummarizeError(tt.err)
			if result != tt.expected {
				t.Errorf("SummarizeError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatEventMessage_PreservesFormatting(t *testing.T) {
	// Test that formatting with multiple args works correctly
	result := FormatEventMessage("pod %s failed: %v", "test-pod", errors.New("crashed"))
	expected := "pod test-pod failed: crashed"
	if result != expected {
		t.Errorf("FormatEventMessage() = %q, want %q", result, expected)
	}
}

func TestFormatErrorEvent_HandlesNilGracefully(t *testing.T) {
	// Ensure nil error doesn't panic
	result := FormatErrorEvent("context", nil)
	if result != "context" {
		t.Errorf("FormatErrorEvent() with nil error = %q, want %q", result, "context")
	}
}
