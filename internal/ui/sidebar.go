package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/luisalfredotejeda/texman/internal/collection"
	"github.com/luisalfredotejeda/texman/internal/responses"
)

// ListItem is one row in the flattened sidebar tree. Collection headers have
// ReqIdx == -1; request rows carry the collection and request indices needed
// to look up the actual data.
type ListItem struct {
	IsCollection bool
	CollIdx      int
	ReqIdx       int
	Label        string
	Method       string
	Collapsed    bool
}

// BuildFlatList produces the ordered, flat list of sidebar rows from the
// current collections and their collapsed/expanded state.
func BuildFlatList(cols []collection.Collection, collapsed map[int]bool) []ListItem {
	var items []ListItem
	for ci, c := range cols {
		items = append(items, ListItem{
			IsCollection: true,
			CollIdx:      ci,
			ReqIdx:       -1,
			Label:        c.Name,
			Collapsed:    collapsed[ci],
		})
		if !collapsed[ci] {
			for ri, r := range c.Requests {
				items = append(items, ListItem{
					IsCollection: false,
					CollIdx:      ci,
					ReqIdx:       ri,
					Label:        r.Name,
					Method:       r.Method,
				})
			}
		}
	}
	return items
}

// RenderSidebar returns the fully styled sidebar panel string.
// width and height are the TOTAL panel dimensions including border.
func RenderSidebar(items []ListItem, cursor int, focused bool, width, height int) string {
	// Inner dimensions after subtracting rounded-border (1 char per side).
	innerW := width - 2
	innerH := height - 2

	if innerW < 4 || innerH < 4 {
		style := PanelStyle
		if focused {
			style = ActivePanelStyle
		}
		return style.Width(max(innerW, 2)).Height(max(innerH, 2)).Render("")
	}

	// Fixed header: title + separator (2 lines).
	const headerLines = 2
	availLines := innerH - headerLines

	// Compute scroll offset so the cursor stays in view.
	scrollStart := 0
	if cursor >= availLines {
		scrollStart = cursor - availLines + 1
	}

	var sb strings.Builder

	// Header
	sb.WriteString(TitleStyle.Render("Collections") + "\n")
	sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")

	// Items
	linesWritten := 0
	for i, item := range items {
		if i < scrollStart {
			continue
		}
		if linesWritten >= availLines {
			break
		}
		sb.WriteString(renderSidebarItem(item, i == cursor, innerW) + "\n")
		linesWritten++
	}

	if len(items) == 0 {
		sb.WriteString(DimStyle.Render("  No collections loaded.") + "\n")
		linesWritten++
	}

	// Pad remaining rows so the panel fills its full height.
	for linesWritten < availLines {
		sb.WriteString("\n")
		linesWritten++
	}

	style := PanelStyle
	if focused {
		style = ActivePanelStyle
	}
	return style.Width(innerW).Height(innerH).Render(sb.String())
}

// RenderResponseSidebar returns a sidebar listing saved response files.
func RenderResponseSidebar(files []responses.File, cursor int, focused bool, width, height int) string {
	innerW := width - 2
	innerH := height - 2

	if innerW < 4 || innerH < 4 {
		style := PanelStyle
		if focused {
			style = ActivePanelStyle
		}
		return style.Width(max(innerW, 2)).Height(max(innerH, 2)).Render("")
	}

	const headerLines = 2
	availLines := innerH - headerLines
	scrollStart := 0
	if cursor >= availLines {
		scrollStart = cursor - availLines + 1
	}

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("Responses") + "\n")
	sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")

	linesWritten := 0
	for i, file := range files {
		if i < scrollStart {
			continue
		}
		if linesWritten >= availLines {
			break
		}
		sb.WriteString(renderResponseFileItem(file, i == cursor, innerW) + "\n")
		linesWritten++
	}

	if len(files) == 0 {
		sb.WriteString(DimStyle.Render("  No response files.") + "\n")
		linesWritten++
	}

	for linesWritten < availLines {
		sb.WriteString("\n")
		linesWritten++
	}

	style := PanelStyle
	if focused {
		style = ActivePanelStyle
	}
	return style.Width(innerW).Height(innerH).Render(sb.String())
}

// renderSidebarItem formats a single sidebar row.
// For the selected row we render with SelectedItemStyle so the background
// spans the full inner width. For unselected rows we apply per-element colour.
func renderSidebarItem(item ListItem, selected bool, innerW int) string {
	if item.IsCollection {
		icon := "▼"
		if item.Collapsed {
			icon = "▶"
		}
		label := icon + " " + item.Label
		if selected {
			return SelectedItemStyle.Width(innerW).Render(label)
		}
		return CollectionStyle.Render(label)
	}

	// Request row
	if selected {
		label := fmt.Sprintf("  %-7s %s", item.Method, truncate(item.Label, innerW-10))
		return SelectedItemStyle.Width(innerW).Render(label)
	}

	method := MethodStyle(item.Method).Render(fmt.Sprintf("%-7s", item.Method))
	name := DimStyle.Render(truncate(item.Label, innerW-10))
	line := "  " + method + " " + name

	// Guard against overly long lines escaping the panel width.
	if lipgloss.Width(line) > innerW {
		line = "  " + method + " " + DimStyle.Render(truncate(item.Label, max(0, innerW-12)))
	}
	return line
}

func renderResponseFileItem(file responses.File, selected bool, innerW int) string {
	size := formatBytes(file.Size)
	maxNameW := innerW - len(size) - 3
	if maxNameW < 4 {
		maxNameW = innerW
	}
	label := fmt.Sprintf("  %s %s", truncate(file.Name, maxNameW), DimStyle.Render(size))
	if selected {
		return SelectedItemStyle.Width(innerW).Render(label)
	}
	return label
}

func formatBytes(size int64) string {
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

// truncate shortens s to at most n visible characters, appending "…" if cut.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
