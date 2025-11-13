package controllers

import (
	"fmt"
	"strings"
)

// EventMessage creates a clean, concise event message.
// It limits message length and removes multi-line content to keep events readable.
const maxEventMessageLength = 256

// FormatEventMessage creates a clean, concise event message suitable for Kubernetes events.
// It sanitizes error messages by:
// - Limiting total length to avoid cluttering kubectl describe output
// - Removing newlines and excessive whitespace
// - Extracting the core error message without stack traces
func FormatEventMessage(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)

	// Remove newlines and normalize whitespace
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.ReplaceAll(msg, "\t", " ")

	// Collapse multiple spaces into one
	for strings.Contains(msg, "  ") {
		msg = strings.ReplaceAll(msg, "  ", " ")
	}

	msg = strings.TrimSpace(msg)

	// Truncate if too long
	if len(msg) > maxEventMessageLength {
		msg = msg[:maxEventMessageLength-3] + "..."
	}

	return msg
}

// FormatErrorEvent creates a clean event message from an error.
// It extracts the root cause and avoids including verbose error chains.
func FormatErrorEvent(context string, err error) string {
	if err == nil {
		return context
	}

	errMsg := err.Error()

	// Extract just the last error in the chain (the root cause)
	// Many Go errors are wrapped and create very long messages
	parts := strings.Split(errMsg, ":")
	if len(parts) > 3 {
		// Take the last meaningful part
		errMsg = strings.TrimSpace(parts[len(parts)-1])
	}

	// Remove common verbose prefixes
	errMsg = strings.TrimPrefix(errMsg, "error ")
	errMsg = strings.TrimPrefix(errMsg, "failed to ")

	return FormatEventMessage("%s: %s", context, errMsg)
}

// SummarizeError returns a short, human-readable error summary.
// This is used when we want just the error type/category, not the full message.
func SummarizeError(err error) string {
	if err == nil {
		return "unknown error"
	}

	errStr := err.Error()

	// Common error patterns to extract
	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "connection refused"):
		return "connection refused"
	case strings.Contains(errStr, "no such host"):
		return "host not found"
	case strings.Contains(errStr, "not found"):
		return "not found"
	case strings.Contains(errStr, "already exists"):
		return "already exists"
	case strings.Contains(errStr, "forbidden"):
		return "forbidden"
	case strings.Contains(errStr, "unauthorized"):
		return "unauthorized"
	case strings.Contains(errStr, "invalid"):
		return "invalid"
	case strings.Contains(errStr, "context canceled"):
		return "canceled"
	case strings.Contains(errStr, "context deadline exceeded"):
		return "deadline exceeded"
	default:
		// Return first sentence or first 50 chars
		if idx := strings.Index(errStr, "."); idx > 0 && idx < 100 {
			return errStr[:idx]
		}
		if len(errStr) > 50 {
			return errStr[:47] + "..."
		}
		return errStr
	}
}
