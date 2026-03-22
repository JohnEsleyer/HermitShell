// Package parser provides XML tag parsing for Hermit agent responses.
//
// Documentation:
// - xml-tags.md: Complete reference for all XML tags (<message>, <terminal>, <give>, <app>, etc.)
package parser

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

type ParsedResponse struct {
	Thought   string           `json:"thought"`
	Message   string           `json:"message"`
	Terminal  string           `json:"terminal"`
	Terminals []string         `json:"terminals,omitempty"`
	System    string           `json:"system,omitempty"`
	Actions   []ParsedAction   `json:"actions"`
	Calendars []ParsedCalendar `json:"calendars,omitempty"`
	Apps      []ParsedApp      `json:"apps,omitempty"`
	Deploys   []string         `json:"deploys,omitempty"`
}

type ParsedAction struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ParsedCalendar struct {
	DateTime string `json:"datetime,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
	ID       string `json:"id,omitempty"`
	Action   string `json:"action,omitempty"` // "create", "list", "delete", "update"
	// Relative time support via <schedule minutes="N" hours="N" days="N">
	ScheduleMinutes int `json:"scheduleMinutes,omitempty"`
	ScheduleHours   int `json:"scheduleHours,omitempty"`
	ScheduleDays    int `json:"scheduleDays,omitempty"`
}

// ParsedApp represents a parsed <app> tag with HTML, CSS, and JS content.
// Docs: See docs/xml-tags.md for <app> tag format and usage.
type ParsedApp struct {
	Name string `json:"name"`
	HTML string `json:"html"`
	CSS  string `json:"css"`
	JS   string `json:"js"`
}

func activeZone(text string) string {
	idx := strings.LastIndex(strings.ToLower(text), "<end>")
	if idx == -1 {
		return text
	}
	// Return everything after the last <end> tag (5 is len("<end>"))
	return text[idx+5:]
}

var (
	thoughtRegex          = regexp.MustCompile(`(?is)<thought>(.*?)</thought>`)
	messageRegex          = regexp.MustCompile(`(?is)<message>(.*?)</message>`)
	terminalRegex         = regexp.MustCompile(`(?is)<terminal>(.*?)</terminal>`)
	systemRegex           = regexp.MustCompile(`(?is)<system>(.*?)</system>`)
	skillRegex            = regexp.MustCompile(`(?is)<skill>(.*?)</skill>`)
	calendarRegex         = regexp.MustCompile(`(?is)<calendar>(.*?)</calendar>`)
	calendarDateRegex     = regexp.MustCompile(`(?is)<date>(.*?)</date>`)
	calendarTimeRegex     = regexp.MustCompile(`(?is)<time>(.*?)</time>`)
	calendarDateTimeRegex = regexp.MustCompile(`(?is)<datetime>(.*?)</datetime>`)
	// Calendar CRUD operations
	calendarListRegex   = regexp.MustCompile(`(?is)<calendar\s+action=["']?list["']?\s*/>`)
	calendarDeleteRegex = regexp.MustCompile(`(?is)<calendar\s+action=["']?delete["']?\s+id=["']?([^"'>]+)["']?\s*/>`)
	calendarUpdateRegex = regexp.MustCompile(`(?is)<calendar\s+action=["']?update["']?\s+id=["']?([^"'>]+)["']?>(.*?)</calendar>`)
	// New tags: <give>, <app>
	giveRegex    = regexp.MustCompile(`(?is)<give>(.*?)</give>`)
	appRegex     = regexp.MustCompile(`(?is)<app\s+name=["']?([^"'>]+)["']?>(.*?)</app>`)
	appHTMLRegex = regexp.MustCompile(`(?is)<html>(.*?)</html>`)
	appCSSRegex  = regexp.MustCompile(`(?is)<style>(.*?)</style>`)
	appJSRegex   = regexp.MustCompile(`(?is)<script>(.*?)</script>`)
	deployRegex  = regexp.MustCompile(`(?is)<deploy>(.*?)</deploy>`)
	// New unified <schedule> tag for relative time scheduling
	// Usage: <schedule minutes="3" hours="1" days="2">reminder text</schedule>
	scheduleMinutesRegex = regexp.MustCompile(`minutes=["']?(\d+)["']?`)
	scheduleHoursRegex   = regexp.MustCompile(`hours=["']?(\d+)["']?`)
	scheduleDaysRegex    = regexp.MustCompile(`days=["']?(\d+)["']?`)
)

// ParseLLMOutput parses XML tags from LLM response.
// Docs: See docs/xml-tags.md for all supported tags.
// Supported tags: <message>, <terminal>, <give>, <app>, <skill>, <calendar>, <thought>, <system>
//
// Rules:
// - <message> tags: Extracted for user transport (Telegram/HermitChat)
// - <thought> tags: Internal only, NOT sent to user
// - Plain text outside tags: Ignored (not sent to user)
// - At least one <message> tag is REQUIRED, otherwise response is rejected
func ParseLLMOutput(text string) ParsedResponse {
	log.Printf("[PARSER] Input text: %s", text)

	text = activeZone(text)
	resp := ParsedResponse{
		Actions:   make([]ParsedAction, 0),
		Terminals: make([]string, 0),
		Apps:      make([]ParsedApp, 0),
		Deploys:   make([]string, 0),
	}

	if m := thoughtRegex.FindStringSubmatch(text); len(m) > 1 {
		resp.Thought = strings.TrimSpace(m[1])
	}

	// Extract <message> tag content
	// Plain text outside tags is IGNORED - only <message> content goes to user
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

	// Handle <give>filename</give>
	giveMatches := giveRegex.FindAllStringSubmatch(text, -1)
	for _, m := range giveMatches {
		if len(m) > 1 {
			filename := strings.TrimSpace(m[1])
			if filename != "" {
				resp.Actions = append(resp.Actions, ParsedAction{Type: "GIVE", Value: filename})
			}
		}
	}

	// Handle legacy <action type="GIVE">...</action> (backward compatibility)
	actionRegex := regexp.MustCompile(`(?is)<action\s+type=["']?GIVE["']?>(.*?)</action>`)
	actionMatches := actionRegex.FindAllStringSubmatch(text, -1)
	for _, m := range actionMatches {
		if len(m) > 1 {
			filename := strings.TrimSpace(m[1])
			if filename != "" {
				resp.Actions = append(resp.Actions, ParsedAction{Type: "GIVE", Value: filename})
			}
		}
	}

	// Handle <app name="app-name">...</app>
	appMatches := appRegex.FindAllStringSubmatch(text, -1)
	for _, m := range appMatches {
		if len(m) > 2 {
			appName := strings.TrimSpace(m[1])
			appContent := m[2]

			parsedApp := ParsedApp{Name: appName}

			// Extract HTML
			if htmlMatch := appHTMLRegex.FindStringSubmatch(appContent); len(htmlMatch) > 1 {
				parsedApp.HTML = strings.TrimSpace(htmlMatch[1])
			}

			// Extract CSS
			if cssMatch := appCSSRegex.FindStringSubmatch(appContent); len(cssMatch) > 1 {
				parsedApp.CSS = strings.TrimSpace(cssMatch[1])
			}

			// Extract JS
			if jsMatch := appJSRegex.FindStringSubmatch(appContent); len(jsMatch) > 1 {
				parsedApp.JS = strings.TrimSpace(jsMatch[1])
			}

			// Fallback: if no specific tags found, treat whole content as HTML
			if parsedApp.HTML == "" && parsedApp.CSS == "" && parsedApp.JS == "" {
				parsedApp.HTML = strings.TrimSpace(appContent)
			}

			if parsedApp.HTML != "" || parsedApp.CSS != "" || parsedApp.JS != "" {
				resp.Apps = append(resp.Apps, parsedApp)
			}
		}
	}

	// Handle <deploy>app-name</deploy>
	deployMatches := deployRegex.FindAllStringSubmatch(text, -1)
	for _, m := range deployMatches {
		if len(m) > 1 {
			appName := strings.TrimSpace(m[1])
			if appName != "" {
				resp.Deploys = append(resp.Deploys, appName)
			}
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

	matches := calendarRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		calendarContent := m[1]

		log.Printf("[PARSER DEBUG] calendarContent: %s", calendarContent)

		// Extract prompt
		promptRegex := regexp.MustCompile(`(?is)<prompt>(.*?)</prompt>`)
		prompt := ""
		if pm := promptRegex.FindStringSubmatch(calendarContent); len(pm) > 1 {
			prompt = strings.TrimSpace(pm[1])
			log.Printf("[PARSER DEBUG] prompt found: %s", prompt)
		}

		// Extract datetime
		datetime := ""
		if dt := calendarDateTimeRegex.FindStringSubmatch(calendarContent); len(dt) > 1 {
			datetime = strings.TrimSpace(dt[1])
			log.Printf("[PARSER DEBUG] datetime found: %s", datetime)
		} else {
			dateVal := ""
			timeVal := ""
			if d := calendarDateRegex.FindStringSubmatch(calendarContent); len(d) > 1 {
				dateVal = strings.TrimSpace(d[1])
			}
			if t := calendarTimeRegex.FindStringSubmatch(calendarContent); len(t) > 1 {
				timeVal = strings.TrimSpace(t[1])
			}
			datetime = strings.TrimSpace(strings.TrimSpace(dateVal) + " " + strings.TrimSpace(timeVal))
		}

		if prompt != "" || datetime != "" {
			resp.Calendars = append(resp.Calendars, ParsedCalendar{DateTime: datetime, Prompt: prompt, Action: "create"})
			log.Printf("[PARSER DEBUG] Added calendar event")
		}
	}

	// Handle calendar list action
	if calendarListRegex.FindStringIndex(text) != nil {
		resp.Calendars = append(resp.Calendars, ParsedCalendar{Action: "list"})
	}

	// Handle unified <schedule> tag for relative time scheduling
	// Usage: <schedule minutes="3">...</schedule>
	//        <schedule hours="1" minutes="30">...</schedule>
	//        <schedule days="2">...</schedule>
	scheduleTagRegex := regexp.MustCompile(`(?is)<schedule\s+([^>]*)>(.*?)</schedule>`)
	scheduleMatches := scheduleTagRegex.FindAllStringSubmatch(text, -1)
	for _, m := range scheduleMatches {
		if len(m) < 3 {
			continue
		}
		attrs := m[1]
		scheduleContent := m[2]

		minutes := 0
		hours := 0
		days := 0

		if minMatch := scheduleMinutesRegex.FindStringSubmatch(attrs); len(minMatch) > 1 {
			if val, err := strconv.Atoi(minMatch[1]); err == nil {
				minutes = val
			}
		}
		if hrMatch := scheduleHoursRegex.FindStringSubmatch(attrs); len(hrMatch) > 1 {
			if val, err := strconv.Atoi(hrMatch[1]); err == nil {
				hours = val
			}
		}
		if dayMatch := scheduleDaysRegex.FindStringSubmatch(attrs); len(dayMatch) > 1 {
			if val, err := strconv.Atoi(dayMatch[1]); err == nil {
				days = val
			}
		}

		// Extract prompt from schedule content
		schedulePrompt := ""
		innerPromptRegex := regexp.MustCompile(`(?is)<prompt>(.*?)</prompt>`)
		if pm := innerPromptRegex.FindStringSubmatch(scheduleContent); len(pm) > 1 {
			schedulePrompt = strings.TrimSpace(pm[1])
		} else {
			// Use the content itself as the prompt if no <prompt> tag
			schedulePrompt = strings.TrimSpace(scheduleContent)
		}

		if schedulePrompt != "" && (minutes > 0 || hours > 0 || days > 0) {
			resp.Calendars = append(resp.Calendars, ParsedCalendar{
				Action:          "create",
				Prompt:          schedulePrompt,
				ScheduleMinutes: minutes,
				ScheduleHours:   hours,
				ScheduleDays:    days,
			})
			log.Printf("[PARSER DEBUG] Added schedule event: minutes=%d, hours=%d, days=%d", minutes, hours, days)
		}
	}

	// Handle calendar delete action
	if m := calendarDeleteRegex.FindStringSubmatch(text); len(m) > 1 {
		resp.Calendars = append(resp.Calendars, ParsedCalendar{Action: "delete", ID: strings.TrimSpace(m[1])})
	}

	// Handle calendar update action
	if m := calendarUpdateRegex.FindStringSubmatch(text); len(m) > 2 {
		updateID := strings.TrimSpace(m[1])
		updateContent := m[2]
		updatePrompt := ""
		updateDatetime := ""
		if p := regexp.MustCompile(`(?is)<prompt>(.*?)</prompt>`).FindStringSubmatch(updateContent); len(p) > 1 {
			updatePrompt = strings.TrimSpace(p[1])
		}
		if dt := calendarDateTimeRegex.FindStringSubmatch(updateContent); len(dt) > 1 {
			updateDatetime = strings.TrimSpace(dt[1])
		}
		resp.Calendars = append(resp.Calendars, ParsedCalendar{Action: "update", ID: updateID, Prompt: updatePrompt, DateTime: updateDatetime})
	}

	return resp
}
