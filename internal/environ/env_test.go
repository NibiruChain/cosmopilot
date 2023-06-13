package environ

import (
	"os"
	"testing"
)

func TestEnviron(t *testing.T) {
	if os.Getenv("integer") != "" {
		t.Fatalf("wrong initialization")
	}

	if os.Getenv("unsigned") != "" {
		t.Fatalf("wrong initialization")
	}

	if os.Getenv("string") != "" {
		t.Fatalf("wrong initialization")
	}

	if GetInt("integer", -1) != -1 {
		t.Fatalf("wanted -1")
	}

	if GetUint64("unsigned", 10) != 10 {
		t.Fatalf("wanted 10")
	}

	if GetString("string", "example") != "example" {
		t.Fatalf("wanted example")
	}

	integer, unsigned, str := "-1", "10", "example"

	if err := os.Setenv("integer", integer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Setenv("unsigned", unsigned); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Setenv("string", str); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if GetInt("integer", -5) != -1 {
		t.Fatalf("wanted -1")
	}

	if GetUint64("unsigned", 15) != 10 {
		t.Fatalf("wanted 10")
	}

	if GetString("string", "invalid") != "example" {
		t.Fatalf("wanted example")
	}
}
