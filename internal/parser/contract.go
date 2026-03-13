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

func activeZone(text string) string {
	idx := strings.LastIndex(strings.ToLower(text), "<end>")
	if idx == -1 {
		return text
	}
	return text[idx+len("<end>"):]
}

var (
	thoughtRegex          = regexp.MustCompile(`(?is)<thought>(.*?)</thought>`)
	messageRegex          = regexp.MustCompile(`(?is)<message>(.*?)</message>`)
	terminalRegex         = regexp.MustCompile(`(?is)<terminal>(.*?)</terminal>`)
	systemRegex           = regexp.MustCompile(`(?is)<system>(.*?)</system>`)
	actionRegex           = regexp.MustCompile(`(?is)<action\s+type=["']?([^"'>]+)["']?>(.*?)</action>`)
	skillRegex            = regexp.MustCompile(`(?is)<skill>(.*?)</skill>`)
	calendarRegex         = regexp.MustCompile(`(?is)<calendar>.*?<prompt>(.*?)</prompt>.*?</calendar>`)
	calendarDateRegex     = regexp.MustCompile(`(?is)<date>(.*?)</date>`)
	calendarTimeRegex     = regexp.MustCompile(`(?is)<time>(.*?)</time>`)
	calendarDateTimeRegex = regexp.MustCompile(`(?is)<datetime>(.*?)</datetime>`)
)

func ParseLLMOutput(text string) ParsedResponse {
	text = activeZone(text)
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

	skillMatches := skillRegex.FindAllStringSubmatch(text, -1)
	for _, m := range skillMatches {
		if len(m) > 1 {
			value := strings.TrimSpace(m[1])
			if value != "" {
				resp.Actions = append(resp.Actions, ParsedAction{Type: "SKILL", Value: value})
			}
		}
	}

	if m := calendarRegex.FindStringSubmatch(text); len(m) > 1 {
		prompt := strings.TrimSpace(m[1])
		datetime := ""
		if dt := calendarDateTimeRegex.FindStringSubmatch(text); len(dt) > 1 {
			datetime = strings.TrimSpace(dt[1])
		} else {
			dateVal := ""
			timeVal := ""
			if d := calendarDateRegex.FindStringSubmatch(text); len(d) > 1 {
				dateVal = strings.TrimSpace(d[1])
			}
			if t := calendarTimeRegex.FindStringSubmatch(text); len(t) > 1 {
				timeVal = strings.TrimSpace(t[1])
			}
			datetime = strings.TrimSpace(strings.TrimSpace(dateVal) + " " + strings.TrimSpace(timeVal))
		}
		if prompt != "" {
			resp.Calendar = &ParsedCalendar{DateTime: datetime, Prompt: prompt}
		}
	}

	return resp
}
