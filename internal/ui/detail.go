package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/luisalfredotejeda/texman/internal/collection"
	"github.com/luisalfredotejeda/texman/internal/httpclient"
)

// spinnerFrames is the Braille-dot spinner sequence used while a request is
// in-flight. The model advances the frame index on every tick.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// RenderDetail returns the right-hand panel string composed of two stacked
// sub-panels: the request details / editor (top, 35% of height) and the
// response (bottom, 65%).
//
// editMode and editorContent are non-zero when the user is editing the
// request; in that case the top panel shows the editor instead of the
// read-only request view.
func RenderDetail(
	req *collection.Request,
	resp *httpclient.Response,
	reqErr error,
	loading bool,
	loadingFrame int,
	responseContent string,
	scrollPct float64,
	focused bool,
	editMode EditMode,
	editorContent string,
	width, height int,
) string {
	requestH := height * 35 / 100
	if requestH < 6 {
		requestH = 6
	}
	responseH := height - requestH
	if responseH < 6 {
		responseH = 6
	}

	reqPanel := renderRequestPanel(req, loading, loadingFrame, editMode, editorContent, focused, width, requestH)
	respPanel := renderResponsePanel(resp, reqErr, responseContent, scrollPct, focused, width, responseH)

	return lipgloss.JoinVertical(lipgloss.Left, reqPanel, respPanel)
}

// RenderSavedResponseDetail returns the right-hand panel used by the saved
// responses browser.
func RenderSavedResponseDetail(path, content string, scrollPct float64, focused bool, width, height int) string {
	innerW := width - 2
	innerH := height - 2

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("Response File") + "\n")
	sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")

	switch {
	case path == "":
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render("  No saved response selected. Run a request first, then switch back here.") + "\n")
	default:
		sb.WriteString(DimStyle.Render("  "+filepath.Base(path)) + "\n")
		sb.WriteString("\n")
		if content == "" {
			sb.WriteString(DimStyle.Render("  (empty)") + "\n")
		} else {
			sb.WriteString(content)
			sb.WriteString("\n")
		}
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  %.0f%%  tab focus  ↑↓ / j k scroll", scrollPct*100)))
	}

	style := PanelStyle
	if focused {
		style = ActivePanelStyle
	}
	return style.Width(innerW).Height(innerH).Render(limitLines(sb.String(), innerH))
}

// ─── Request panel ──────────────────────────────────────────────────────────

func renderRequestPanel(
	req *collection.Request,
	loading bool,
	spinFrame int,
	editMode EditMode,
	editorContent string,
	focused bool,
	width, height int,
) string {
	innerW := width - 2
	innerH := height - 2

	var sb strings.Builder

	if editMode != EditModeNone && editMode != EditModeDeleteConfirm {
		// ── Editor overlay ──────────────────────────────────────────────────
		title, hint := editTitleHint(editMode)
		sb.WriteString(TitleStyle.Render(title) + "\n")
		sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")
		sb.WriteString(DimStyle.Render(hint) + "\n")
		sb.WriteString("\n")
		sb.WriteString(editorContent)
	} else {
		// ── Normal read-only view ────────────────────────────────────────────
		sb.WriteString(TitleStyle.Render("Request") + "\n")
		sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")

		if req == nil {
			sb.WriteString("\n")
			sb.WriteString(DimStyle.Render("  Select a request from the sidebar and press Enter to run it.") + "\n")
		} else {
			method := MethodStyle(req.Method).Render(req.Method)
			sb.WriteString(method + "  " + URLStyle.Render(req.URL) + "\n")
			sb.WriteString("\n")

			if len(req.Headers) > 0 {
				sb.WriteString(DimStyle.Render("  Headers:") + "\n")
				for _, k := range sortedStringKeys(req.Headers) {
					line := "    " + HeaderKeyStyle.Render(k+":") + " " + HeaderValStyle.Render(req.Headers[k])
					sb.WriteString(line + "\n")
				}
			}

			if req.Body != "" {
				sb.WriteString("\n")
				sb.WriteString(DimStyle.Render("  Body:") + "\n")
				lines := strings.SplitN(req.Body, "\n", 5)
				for i, l := range lines {
					if i == 4 {
						sb.WriteString(DimStyle.Render("  ...") + "\n")
						break
					}
					sb.WriteString("  " + l + "\n")
				}
			}

			if loading {
				sb.WriteString("\n")
				spinner := spinnerFrames[spinFrame%len(spinnerFrames)]
				sb.WriteString(LoadingStyle.Render("  "+spinner+" Executing…") + "\n")
			}
		}
	}

	style := PanelStyle
	if focused {
		style = ActivePanelStyle
	}
	return style.Width(innerW).Height(innerH).Render(limitLines(sb.String(), innerH))
}

// editTitleHint returns the panel title and one-line key hint for a given mode.
func editTitleHint(mode EditMode) (title, hint string) {
	switch mode {
	case EditModeBody:
		return "Edit Body", "  ctrl+s save  esc cancel"
	case EditModeHeaderList:
		return "Edit Headers", "  enter edit  a add  d delete  ctrl+s save  esc cancel"
	case EditModeHeaderValue, EditModeNewHeaderValue:
		return "Edit Headers", "  enter confirm  esc back  ctrl+s save"
	case EditModeNewHeaderKey:
		return "Edit Headers", "  enter confirm key  esc back  ctrl+s save"
	case EditModeNewReqName:
		return "New Request  (1/3)  Name", "  enter next  esc cancel"
	case EditModeNewReqMethod:
		return "New Request  (2/3)  Method", "  enter next  esc back"
	case EditModeNewReqURL:
		return "New Request  (3/3)  URL", "  enter add request  esc back"
	case EditModeMethod:
		return "Edit Method", "  ctrl+s save  esc cancel"
	case EditModeURL:
		return "Edit URL", "  ctrl+s save  esc cancel"
	case EditModeRenameCollName:
		return "Rename Collection", "  enter/ctrl+s save  esc cancel"
	case EditModeDeleteCollConfirm:
		return "Delete Collection", "  y confirm  any other key cancel"
	case EditModeImportCollPath:
		return "Import Collection", "  enter import  esc cancel"
	case EditModeExportCollPath:
		return "Export Collection", "  enter export  esc cancel"
	}
	return "Edit", ""
}

// ─── Response panel ─────────────────────────────────────────────────────────

func renderResponsePanel(
	resp *httpclient.Response,
	reqErr error,
	viewportContent string,
	scrollPct float64,
	focused bool,
	width, height int,
) string {
	innerW := width - 2
	innerH := height - 2

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("Response") + "\n")
	sb.WriteString(SeparatorStyle.Render(strings.Repeat("─", innerW)) + "\n")

	switch {
	case reqErr != nil:
		sb.WriteString("\n")
		sb.WriteString(ErrorStyle.Render("  Error: "+reqErr.Error()) + "\n")

	case resp == nil:
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render("  No response yet. Press Enter to execute the selected request.") + "\n")

	default:
		statusStr := StatusStyle(resp.StatusCode).Render(resp.Status)
		durStr := DimStyle.Render(fmt.Sprintf("  (%dms)", resp.Duration.Milliseconds()))
		sb.WriteString("  " + statusStr + durStr + "\n")
		sb.WriteString("\n")
		sb.WriteString(viewportContent)
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  %.0f%%  ↑↓ / j k to scroll", scrollPct*100)))
	}

	style := PanelStyle
	if focused {
		style = ActivePanelStyle
	}
	return style.Width(innerW).Height(innerH).Render(limitLines(sb.String(), innerH))
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedHeaderKeys returns keys of an http.Header map in lexicographic order.
func SortedHeaderKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func limitLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
