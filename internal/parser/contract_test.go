package parser

import "testing"

func TestParseLLMOutputMultipleTerminalsAndSystem(t *testing.T) {
	input := `<terminal>ls -la</terminal><terminal>pwd</terminal><system>time</system><message>Hello</message>`
	parsed := ParseLLMOutput(input)

	if parsed.Terminal != "ls -la" {
		t.Fatalf("expected first terminal to be ls -la, got %q", parsed.Terminal)
	}
	if len(parsed.Terminals) != 2 {
		t.Fatalf("expected 2 terminals, got %d", len(parsed.Terminals))
	}
	if parsed.Terminals[1] != "pwd" {
		t.Fatalf("expected second terminal to be pwd, got %q", parsed.Terminals[1])
	}
	if parsed.System != "time" {
		t.Fatalf("expected system tag to be time, got %q", parsed.System)
	}
}
