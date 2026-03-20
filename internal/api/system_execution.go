package api

import (
	"fmt"
	"strings"

	"github.com/JohnEsleyer/HermitShell/internal/parser"
)

func hasSystemExecutionInput(parsed parser.ParsedResponse) bool {
	if parsed.Thought != "" || parsed.Message != "" || parsed.System != "" {
		return true
	}
	if len(parsed.Terminals) > 0 || len(parsed.Actions) > 0 {
		return true
	}
	if len(parsed.Calendars) > 0 || len(parsed.Apps) > 0 || len(parsed.Deploys) > 0 {
		return true
	}
	return false
}

func describeParsedTags(parsed parser.ParsedResponse) string {
	tags := make([]string, 0)
	appendTag := func(tag string) {
		for _, existing := range tags {
			if existing == tag {
				return
			}
		}
		tags = append(tags, tag)
	}

	if parsed.Message != "" {
		appendTag("<message>")
	}
	if len(parsed.Terminals) > 0 {
		appendTag("<terminal>")
	}
	if parsed.System != "" {
		appendTag("<system>")
	}
	if parsed.Thought != "" {
		appendTag("<thought>")
	}
	if len(parsed.Calendars) > 0 {
		appendTag("<calendar>")
	}
	if len(parsed.Apps) > 0 {
		appendTag("<app>")
	}
	if len(parsed.Deploys) > 0 {
		appendTag("<deploy>")
	}
	for _, action := range parsed.Actions {
		switch strings.ToUpper(action.Type) {
		case "GIVE":
			appendTag("<give>")
		case "SKILL":
			appendTag("<skill>")
		default:
			appendTag("<action>")
		}
	}

	if len(tags) == 0 {
		return "Processed direct system input."
	}
	return "Processed tags: " + strings.Join(tags, ", ")
}

func extractExecutionFiles(parsed parser.ParsedResponse) []string {
	files := make([]string, 0)
	for _, action := range parsed.Actions {
		if strings.EqualFold(action.Type, "GIVE") && action.Value != "" {
			files = append(files, action.Value)
		}
	}
	return files
}

func formatSystemExecutionResponse(parsed parser.ParsedResponse, feedback []map[string]interface{}) string {
	lines := []string{"System response:"}

	if parsed.Message != "" {
		lines = append(lines, fmt.Sprintf("- Message queued: %q", parsed.Message))
	}
	if files := extractExecutionFiles(parsed); len(files) > 0 {
		lines = append(lines, "- Files queued: "+strings.Join(files, ", "))
	}

	for _, effect := range feedback {
		if action, ok := effect["action"].(string); ok {
			status := fmt.Sprintf("%v", effect["status"])
			switch action {
			case "MESSAGE":
				lines = append(lines, "- Message delivery: "+status)
			case "GIVE":
				file := fmt.Sprintf("%v", effect["file"])
				lines = append(lines, fmt.Sprintf("- File transfer: %s (%s)", file, status))
			case "SKILL":
				lines = append(lines, fmt.Sprintf("- Skill action: %v (%s)", effect["skill"], status))
			case "APP", "DEPLOY":
				subject := fmt.Sprintf("%v", effect["app"])
				lines = append(lines, fmt.Sprintf("- %s: %s (%s)", action, subject, status))
			case "CALENDAR", "CALENDAR_LIST", "CALENDAR_DELETE", "CALENDAR_UPDATE":
				lines = append(lines, fmt.Sprintf("- %s: %s", action, status))
			default:
				lines = append(lines, fmt.Sprintf("- %s: %s", action, status))
			}
			if errValue, ok := effect["error"]; ok && fmt.Sprintf("%v", errValue) != "" {
				lines = append(lines, fmt.Sprintf("  Error: %v", errValue))
			}
			continue
		}

		if terminal, ok := effect["terminal"].(string); ok {
			status := fmt.Sprintf("%v", effect["status"])
			lines = append(lines, fmt.Sprintf("- Terminal: %s (%s)", terminal, status))
			output := strings.TrimSpace(fmt.Sprintf("%v", effect["output"]))
			if output != "" {
				lines = append(lines, "  Output: "+output)
			}
			continue
		}

		if systemName, ok := effect["system"].(string); ok {
			lines = append(lines, fmt.Sprintf("- System %s: %v", systemName, effect["value"]))
		}
	}

	if len(lines) == 1 {
		lines = append(lines, "- Execution completed.")
	}

	return strings.Join(lines, "\n")
}
