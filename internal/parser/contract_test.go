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

func TestParseLLMOutputSkillTag(t *testing.T) {
	input := `<message>Loading skill</message><skill>remotion.md</skill>`
	parsed := ParseLLMOutput(input)

	if len(parsed.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(parsed.Actions))
	}
	if parsed.Actions[0].Type != "SKILL" {
		t.Fatalf("expected action type SKILL, got %q", parsed.Actions[0].Type)
	}
	if parsed.Actions[0].Value != "remotion.md" {
		t.Fatalf("expected skill remotion.md, got %q", parsed.Actions[0].Value)
	}
}

func TestParseLLMOutputActiveZoneAfterEnd(t *testing.T) {
	input := `<message>Old</message><terminal>echo old</terminal><end><message>New</message><terminal>echo new</terminal>`
	parsed := ParseLLMOutput(input)

	if parsed.Message != "New" {
		t.Fatalf("expected message from active zone, got %q", parsed.Message)
	}
	if len(parsed.Terminals) != 1 || parsed.Terminals[0] != "echo new" {
		t.Fatalf("expected only new terminal, got %#v", parsed.Terminals)
	}
}

func TestParseLLMOutputCalendarDateAndTime(t *testing.T) {
	input := `<calendar><date>2026-03-14</date><time>08:00</time><prompt>Check logs</prompt></calendar>`
	parsed := ParseLLMOutput(input)

	if parsed.Calendar == nil {
		t.Fatal("expected calendar to be parsed")
	}
	if parsed.Calendar.DateTime != "2026-03-14 08:00" {
		t.Fatalf("expected datetime join, got %q", parsed.Calendar.DateTime)
	}
	if parsed.Calendar.Prompt != "Check logs" {
		t.Fatalf("expected prompt, got %q", parsed.Calendar.Prompt)
	}
}
