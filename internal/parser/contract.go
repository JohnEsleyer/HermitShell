package parser

import (
	"regexp"
	"strings"
)

type ParsedResponse struct {
	Thought   string          `json:"thought"`
	Message   string          `json:"message"`
	Terminal  string          `json:"terminal"`
	Terminals []string        `json:"terminals,omitempty"`
	System    string          `json:"system,omitempty"`
	Actions   []ParsedAction  `json:"actions"`
	Calendar  *ParsedCalendar `json:"calendar,omitempty"`
}

type ParsedAction struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ParsedCalendar struct {
	DateTime string `json:"datetime"`
	Prompt   string `json:"prompt"`
}

var (
	thoughtRegex  = regexp.MustCompile(`(?is)<thought>(.*?)</thought>`)
	messageRegex  = regexp.MustCompile(`(?is)<message>(.*?)</message>`)
	terminalRegex = regexp.MustCompile(`(?is)<terminal>(.*?)</terminal>`)
	systemRegex   = regexp.MustCompile(`(?is)<system>(.*?)</system>`)
	actionRegex   = regexp.MustCompile(`(?is)<action\s+type=["']?([^"'>]+)["']?>(.*?)</action>`)
	calendarRegex = regexp.MustCompile(`(?is)<calendar>\s*<datetime>(.*?)</datetime>\s*<prompt>(.*?)</prompt>\s*</calendar>`)
)

func ParseLLMOutput(text string) ParsedResponse {
	resp := ParsedResponse{
		Actions:   make([]ParsedAction, 0),
		Terminals: make([]string, 0),
	}

	if m := thoughtRegex.FindStringSubmatch(text); len(m) > 1 {
		resp.Thought = strings.TrimSpace(m[1])
	}

	if m := messageRegex.FindStringSubmatch(text); len(m) > 1 {
		resp.Message = strings.TrimSpace(m[1])
	}

	terminalMatches := terminalRegex.FindAllStringSubmatch(text, -1)
	for _, m := range terminalMatches {
		if len(m) > 1 {
			cmd := strings.TrimSpace(m[1])
			if cmd != "" {
				resp.Terminals = append(resp.Terminals, cmd)
			}
		}
	}
	if len(resp.Terminals) > 0 {
		resp.Terminal = resp.Terminals[0]
	}

	if m := systemRegex.FindStringSubmatch(text); len(m) > 1 {
		resp.System = strings.TrimSpace(m[1])
	}

	actionMatches := actionRegex.FindAllStringSubmatch(text, -1)
	for _, m := range actionMatches {
		if len(m) > 2 {
			resp.Actions = append(resp.Actions, ParsedAction{
				Type:  strings.ToUpper(strings.TrimSpace(m[1])),
				Value: strings.TrimSpace(m[2]),
			})
		}
	}

	if m := calendarRegex.FindStringSubmatch(text); len(m) > 2 {
		resp.Calendar = &ParsedCalendar{
			DateTime: strings.TrimSpace(m[1]),
			Prompt:   strings.TrimSpace(m[2]),
		}
	}

	return resp
}
