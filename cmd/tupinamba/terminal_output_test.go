package main

import (
	"strings"
	"testing"
)

func TestDisplayTextNeutralizesTerminalControls(t *testing.T) {
	got := displayText("Coffee\x1b[2J\x07\r\nArabica")
	if strings.ContainsAny(got, "\x1b\x07\r\n") {
		t.Fatalf("terminal control characters were preserved: %q", got)
	}
	if !strings.Contains(got, "Coffee") || !strings.Contains(got, "Arabica") {
		t.Fatalf("display text lost readable content: %q", got)
	}
}
