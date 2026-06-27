// Package model contains the Bubble Tea model that drives the entire TUI.
package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/luisalfredotejeda/texman/internal/collection"
	"github.com/luisalfredotejeda/texman/internal/httpclient"
	"github.com/luisalfredotejeda/texman/internal/responses"
	"github.com/luisalfredotejeda/texman/internal/ui"
)

// ─── Focus ──────────────────────────────────────────────────────────────────

type Focus int

const (
	FocusSidebar Focus = iota
	FocusDetail
)

type ViewMode int

const (
	ViewCollections ViewMode = iota
	ViewResponses
)

// ─── Internal messages ──────────────────────────────────────────────────────

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// clearStatusMsg is dispatched 2 s after a successful save to clear the
// status bar notification.
type clearStatusMsg struct{}

func clearStatus() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// ─── Model ──────────────────────────────────────────────────────────────────

type Model struct {
	// Collection data (parallel: filePaths[i] is the source file for collections[i])
	collections []collection.Collection
	filePaths   []string
	flatList    []ui.ListItem
	cursor      int
	collapsed   map[int]bool

	// Focus
	focus    Focus
	viewMode ViewMode

	// HTTP state
	loading      bool
	loadingFrame int
	response     *httpclient.Response
	err          error

	// Scrollable response body
	responseVP viewport.Model

	// ── Edit state ──────────────────────────────────────────────────────────
	editMode ui.EditMode

	// Body editor (textarea, active during EditModeBody)
	bodyTA textarea.Model

	// Header editor (textinput, active during EditModeHeader{Value,NewHeaderKey,NewHeaderValue})
	headerTI       textinput.Model
	editHeaderKey  string // key currently being edited / newly created
	isNewHeader    bool
	headerCursor   int
	pendingHeaders map[string]string // working copy of the request's headers
	pendingHdrKeys []string          // sorted keys of pendingHeaders

	// Status bar notification (auto-cleared after 2 s)
	statusMsg string

	// New-request wizard state
	newReqName       string
	newReqMethod     string
	newReqTargetColl int

	// New-collection wizard state
	newCollName string

	// Index of the collection being renamed/deleted
	renameCollIdx int

	// Directory where collection JSON files live
	collectionsDir string

	// Directory where response snapshots are saved
	responsesDir string

	// Name of the most recently dispatched request (used when saving the response)
	lastReqName string

	// Saved response browser state
	responseFiles           []responses.File
	responseCursor          int
	selectedResponsePath    string
	selectedResponseContent string
	responseFileVP          viewport.Model

	// Terminal dimensions
	width, height int

	// Pre-computed layout values
	sidebarW  int
	detailW   int
	totalH    int
	requestH  int
	responseH int
}

// New creates a Model pre-loaded with collections and their source file paths.
// collectionsDir is where new collection JSON files will be created.
// responsesDir is where HTTP response snapshots will be saved.
func New(cols []collection.Collection, paths []string, collectionsDir, responsesDir string) Model {
	m := Model{
		collections:    cols,
		filePaths:      paths,
		collapsed:      make(map[int]bool),
		focus:          FocusSidebar,
		width:          80,
		height:         24,
		collectionsDir: collectionsDir,
		responsesDir:   responsesDir,
	}
	m.recalcLayout()
	m.flatList = ui.BuildFlatList(cols, m.collapsed)
	return m
}

// ─── Bubble Tea interface ────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles all incoming messages and returns the next model + commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Terminal resize ────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.applySize(msg.Width, msg.Height)
		cmds = append(cmds, tea.ClearScreen)

	// ── Spinner tick ───────────────────────────────────────────────────────
	case tickMsg:
		if m.loading {
			m.loadingFrame = (m.loadingFrame + 1) % 10
			cmds = append(cmds, tick())
		}

	// ── HTTP response ──────────────────────────────────────────────────────
	case httpclient.ResponseMsg:
		m.loading = false
		m.response = msg.Resp
		m.err = nil
		if msg.Resp != nil {
			m.responseVP.SetContent(m.formatViewportContent(msg.Resp))
			m.responseVP.GotoTop()
			dir := m.responsesDir
			if dir == "" {
				dir = "./responses"
			}
			if path, err := responses.Save(dir, m.lastReqName, msg.Resp); err != nil {
				m.statusMsg = "save error: " + err.Error()
			} else {
				m.statusMsg = "saved → " + path
				cmds = append(cmds, clearStatus())
			}
		}

	// ── HTTP error ─────────────────────────────────────────────────────────
	case httpclient.ErrMsg:
		m.loading = false
		m.err = msg.Err
		m.response = nil
		m.responseVP.SetContent("")

	// ── Auto-clear status bar ──────────────────────────────────────────────
	case clearStatusMsg:
		m.statusMsg = ""

	// ── Keyboard ──────────────────────────────────────────────────────────
	case tea.KeyMsg:
		if m.editMode != ui.EditModeNone {
			// ── ctrl+s / esc are global within edit mode ──────────────────
			switch msg.String() {
			case "ctrl+s":
				switch m.editMode {
				case ui.EditModeBody:
					m.saveBody()
					cmds = append(cmds, clearStatus())
				case ui.EditModeHeaderList,
					ui.EditModeHeaderValue,
					ui.EditModeNewHeaderKey,
					ui.EditModeNewHeaderValue:
					m.saveHeaders()
					cmds = append(cmds, clearStatus())
				case ui.EditModeMethod:
					m.saveMethod()
					cmds = append(cmds, clearStatus())
				case ui.EditModeURL:
					m.saveURL()
					cmds = append(cmds, clearStatus())
				case ui.EditModeRenameCollName:
					m.saveCollectionRename()
					cmds = append(cmds, clearStatus())
				case ui.EditModeImportCollPath:
					m.importCollectionFromInput()
					cmds = append(cmds, clearStatus())
				case ui.EditModeExportCollPath:
					m.exportCollectionToInput()
					cmds = append(cmds, clearStatus())
				}
				// new-req wizard and delete-confirm ignore ctrl+s

			case "esc":
				switch m.editMode {
				// Header sub-modes → step back to header list.
				case ui.EditModeHeaderValue,
					ui.EditModeNewHeaderKey,
					ui.EditModeNewHeaderValue:
					m.headerTI.Blur()
					m.editMode = ui.EditModeHeaderList

				// New-req wizard: step back or cancel.
				case ui.EditModeNewReqMethod:
					m.headerTI.Blur()
					cmd := m.enterNewReqNameStep()
					cmds = append(cmds, cmd)
				case ui.EditModeNewReqURL:
					m.headerTI.Blur()
					cmd := m.enterNewReqMethodStep()
					cmds = append(cmds, cmd)

				// Everything else (top-level edit, delete confirm, name step, method, new coll) → cancel.
				default:
					m.headerTI.Blur()
					m.bodyTA.Blur()
					m.editMode = ui.EditModeNone
					m.pendingHeaders = nil
					m.pendingHdrKeys = nil
					m.newReqName = ""
					m.newReqMethod = ""
					m.newCollName = ""
				}

			default:
				// ── Mode-specific key handling ────────────────────────────
				switch m.editMode {

				case ui.EditModeBody:
					var cmd tea.Cmd
					m.bodyTA, cmd = m.bodyTA.Update(msg)
					cmds = append(cmds, cmd)

				case ui.EditModeHeaderList:
					switch msg.String() {
					case "j", "down":
						if m.headerCursor < len(m.pendingHdrKeys)-1 {
							m.headerCursor++
						}
					case "k", "up":
						if m.headerCursor > 0 {
							m.headerCursor--
						}
					case "a":
						cmd := m.enterNewHeaderKey()
						cmds = append(cmds, cmd)
					case "d":
						m.deleteSelectedHeader()
					case "enter":
						if len(m.pendingHdrKeys) > 0 {
							cmd := m.enterHeaderValueEdit()
							cmds = append(cmds, cmd)
						}
					}

				case ui.EditModeHeaderValue, ui.EditModeNewHeaderValue:
					switch msg.String() {
					case "enter":
						m.confirmHeaderValue()
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				case ui.EditModeNewHeaderKey:
					switch msg.String() {
					case "enter":
						cmd := m.confirmNewHeaderKey()
						cmds = append(cmds, cmd)
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				// ── New-request wizard ────────────────────────────────────────
				case ui.EditModeNewReqName:
					switch msg.String() {
					case "enter":
						if name := strings.TrimSpace(m.headerTI.Value()); name != "" {
							m.newReqName = name
							m.headerTI.Blur()
							cmd := m.enterNewReqMethodStep()
							cmds = append(cmds, cmd)
						}
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				case ui.EditModeNewReqMethod:
					switch msg.String() {
					case "enter":
						method := strings.ToUpper(strings.TrimSpace(m.headerTI.Value()))
						if method == "" {
							method = "GET"
						}
						m.newReqMethod = method
						m.headerTI.Blur()
						cmd := m.enterNewReqURLStep()
						cmds = append(cmds, cmd)
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				case ui.EditModeNewReqURL:
					switch msg.String() {
					case "enter":
						if url := strings.TrimSpace(m.headerTI.Value()); url != "" {
							m.addNewRequest(url)
							cmds = append(cmds, clearStatus())
						}
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				// ── New-collection wizard ─────────────────────────────────────
				case ui.EditModeNewCollName:
					switch msg.String() {
					case "enter":
						if name := strings.TrimSpace(m.headerTI.Value()); name != "" {
							m.addNewCollection(name)
							cmds = append(cmds, clearStatus())
						}
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				// ── Rename collection ─────────────────────────────────────────
				case ui.EditModeRenameCollName:
					switch msg.String() {
					case "enter":
						m.saveCollectionRename()
						cmds = append(cmds, clearStatus())
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				// ── Delete collection confirmation ────────────────────────────
				case ui.EditModeImportCollPath:
					switch msg.String() {
					case "enter":
						m.importCollectionFromInput()
						cmds = append(cmds, clearStatus())
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				case ui.EditModeExportCollPath:
					switch msg.String() {
					case "enter":
						m.exportCollectionToInput()
						cmds = append(cmds, clearStatus())
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				case ui.EditModeDeleteCollConfirm:
					switch msg.String() {
					case "y", "Y":
						m.deleteSelectedCollection()
						cmds = append(cmds, clearStatus())
					default:
						m.editMode = ui.EditModeNone
					}

				// ── Delete confirmation ───────────────────────────────────────
				case ui.EditModeDeleteConfirm:
					switch msg.String() {
					case "y", "Y":
						m.deleteSelectedRequest()
						cmds = append(cmds, clearStatus())
					default:
						m.editMode = ui.EditModeNone
					}

				// ── Method editor ─────────────────────────────────────────────
				case ui.EditModeMethod:
					switch msg.String() {
					case "enter":
						m.saveMethod()
						cmds = append(cmds, clearStatus())
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}

				// ── URL editor ────────────────────────────────────────────────
				case ui.EditModeURL:
					switch msg.String() {
					case "enter":
						m.saveURL()
						cmds = append(cmds, clearStatus())
					default:
						var cmd tea.Cmd
						m.headerTI, cmd = m.headerTI.Update(msg)
						cmds = append(cmds, cmd)
					}
				}
			}
		} else {
			// ── Normal navigation (no edit mode active) ───────────────────
			if m.viewMode == ViewResponses {
				switch msg.String() {
				case "ctrl+c", "q":
					return m, tea.Quit

				case "v":
					m.viewMode = ViewCollections
					m.focus = FocusSidebar

				case "tab":
					if m.focus == FocusSidebar {
						m.focus = FocusDetail
					} else {
						m.focus = FocusSidebar
					}

				case "up", "k":
					if m.focus == FocusSidebar {
						m.moveResponseCursor(-1)
					} else {
						var cmd tea.Cmd
						m.responseFileVP, cmd = m.responseFileVP.Update(msg)
						cmds = append(cmds, cmd)
					}

				case "down", "j":
					if m.focus == FocusSidebar {
						m.moveResponseCursor(1)
					} else {
						var cmd tea.Cmd
						m.responseFileVP, cmd = m.responseFileVP.Update(msg)
						cmds = append(cmds, cmd)
					}

				case "pgup", "b":
					if m.focus == FocusDetail {
						var cmd tea.Cmd
						m.responseFileVP, cmd = m.responseFileVP.Update(msg)
						cmds = append(cmds, cmd)
					}

				case "pgdown", "f":
					if m.focus == FocusDetail {
						var cmd tea.Cmd
						m.responseFileVP, cmd = m.responseFileVP.Update(msg)
						cmds = append(cmds, cmd)
					}

				case "enter":
					m.loadSelectedResponseFile()

				case "d":
					m.deleteSelectedResponseFile()
					cmds = append(cmds, clearStatus())
				}
				break
			}

			switch msg.String() {

			case "ctrl+c", "q":
				return m, tea.Quit

			case "v":
				m.enterResponsesView()

			case "tab":
				if m.focus == FocusSidebar {
					m.focus = FocusDetail
				} else {
					m.focus = FocusSidebar
				}

			case "up", "k":
				if m.focus == FocusSidebar {
					if m.cursor > 0 {
						m.cursor--
					}
				} else {
					var cmd tea.Cmd
					m.responseVP, cmd = m.responseVP.Update(msg)
					cmds = append(cmds, cmd)
				}

			case "down", "j":
				if m.focus == FocusSidebar {
					if m.cursor < len(m.flatList)-1 {
						m.cursor++
					}
				} else {
					var cmd tea.Cmd
					m.responseVP, cmd = m.responseVP.Update(msg)
					cmds = append(cmds, cmd)
				}

			case "pgup", "b":
				if m.focus == FocusDetail {
					var cmd tea.Cmd
					m.responseVP, cmd = m.responseVP.Update(msg)
					cmds = append(cmds, cmd)
				}

			case "pgdown", "f":
				if m.focus == FocusDetail {
					var cmd tea.Cmd
					m.responseVP, cmd = m.responseVP.Update(msg)
					cmds = append(cmds, cmd)
				}

			case " ":
				if m.focus == FocusSidebar {
					m.toggleCollapse()
				}

			case "enter":
				if len(m.flatList) == 0 {
					break
				}
				item := m.flatList[m.cursor]
				if item.IsCollection {
					m.toggleCollapse()
				} else if m.focus == FocusSidebar {
					req := m.selectedRequest()
					if req != nil && !m.loading {
						m.loading = true
						m.response = nil
						m.err = nil
						m.loadingFrame = 0
						m.lastReqName = req.Name
						m.responseVP.SetContent("")
						cmds = append(cmds, httpclient.Execute(*req), tick())
					}
				}

			// r → run request OR rename collection (depends on cursor position).
			case "r":
				if len(m.flatList) > 0 && m.flatList[m.cursor].IsCollection {
					cmd := m.enterRenameCollection()
					cmds = append(cmds, cmd)
				} else {
					req := m.selectedRequest()
					if req != nil && !m.loading {
						m.loading = true
						m.response = nil
						m.err = nil
						m.loadingFrame = 0
						m.lastReqName = req.Name
						m.responseVP.SetContent("")
						cmds = append(cmds, httpclient.Execute(*req), tick())
					}
				}

			// ── Edit keys ───────────────────────────────────────────────
			case "e":
				// Enter body edit mode for the selected request.
				if len(m.flatList) > 0 && !m.flatList[m.cursor].IsCollection {
					cmd := m.enterBodyEdit()
					cmds = append(cmds, cmd)
				}

			case "h":
				// Enter header list edit mode for the selected request.
				if len(m.flatList) > 0 && !m.flatList[m.cursor].IsCollection {
					m.enterHeaderList()
				}

			case "m":
				// Edit the HTTP method of the selected request.
				if len(m.flatList) > 0 && !m.flatList[m.cursor].IsCollection {
					cmd := m.enterMethodEdit()
					cmds = append(cmds, cmd)
				}

			case "u":
				// Edit the URL of the selected request.
				if len(m.flatList) > 0 && !m.flatList[m.cursor].IsCollection {
					cmd := m.enterURLEdit()
					cmds = append(cmds, cmd)
				}

			case "n":
				// Start the new-request wizard.
				if len(m.flatList) > 0 {
					cmd := m.enterNewReq()
					cmds = append(cmds, cmd)
				}

			case "C":
				// Start the new-collection wizard.
				cmd := m.enterNewColl()
				cmds = append(cmds, cmd)

			case "i", "I":
				// Import a collection from a JSON file.
				cmd := m.enterImportCollection()
				cmds = append(cmds, cmd)

			case "x", "X":
				// Export the selected collection to a JSON file.
				cmd := m.enterExportCollection()
				cmds = append(cmds, cmd)

			case "d":
				// Prompt to delete the selected request.
				if len(m.flatList) > 0 && !m.flatList[m.cursor].IsCollection {
					m.editMode = ui.EditModeDeleteConfirm
				}

			case "D":
				// Prompt to delete the selected collection.
				if len(m.flatList) > 0 && m.flatList[m.cursor].IsCollection {
					m.renameCollIdx = m.flatList[m.cursor].CollIdx
					m.editMode = ui.EditModeDeleteCollConfirm
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the full TUI screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initialising…"
	}

	if m.viewMode == ViewResponses {
		sidebar := ui.RenderResponseSidebar(
			m.responseFiles,
			m.responseCursor,
			m.focus == FocusSidebar,
			m.sidebarW,
			m.totalH,
		)
		detail := ui.RenderSavedResponseDetail(
			m.selectedResponsePath,
			m.responseFileVP.View(),
			m.responseFileVP.ScrollPercent(),
			m.focus == FocusDetail,
			m.detailW,
			m.totalH,
		)
		layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)
		return layout + "\n" + m.helpLine()
	}

	sidebar := ui.RenderSidebar(
		m.flatList,
		m.cursor,
		m.focus == FocusSidebar,
		m.sidebarW,
		m.totalH,
	)

	// Build the pre-rendered editor content for the request panel.
	var editorContent string
	switch m.editMode {
	case ui.EditModeBody:
		editorContent = m.bodyTA.View()
	case ui.EditModeHeaderList,
		ui.EditModeHeaderValue,
		ui.EditModeNewHeaderKey,
		ui.EditModeNewHeaderValue:
		editorContent = m.renderHeaderEditor()
	case ui.EditModeNewReqName,
		ui.EditModeNewReqMethod,
		ui.EditModeNewReqURL:
		editorContent = m.renderNewReqEditor()
	case ui.EditModeNewCollName:
		editorContent = m.renderNewCollEditor()
	case ui.EditModeRenameCollName:
		editorContent = m.renderRenameCollEditor()
	case ui.EditModeImportCollPath:
		editorContent = m.renderImportCollEditor()
	case ui.EditModeExportCollPath:
		editorContent = m.renderExportCollEditor()
	case ui.EditModeDeleteCollConfirm:
		editorContent = m.renderDeleteCollEditor()
	case ui.EditModeMethod:
		editorContent = "  " + m.headerTI.View() + "\n"
	case ui.EditModeURL:
		editorContent = "  " + m.headerTI.View() + "\n"
	}

	detail := ui.RenderDetail(
		m.selectedRequest(),
		m.response,
		m.err,
		m.loading,
		m.loadingFrame,
		m.responseVP.View(),
		m.responseVP.ScrollPercent(),
		m.focus == FocusDetail,
		m.editMode,
		editorContent,
		m.detailW,
		m.totalH,
	)

	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)

	return layout + "\n" + m.helpLine()
}

// ─── Edit helpers ────────────────────────────────────────────────────────────

// editTarget resolves the currently selected collection/request indices.
// Returns (0, 0, false) if the cursor is on a collection header.
func (m Model) editTarget() (collIdx, reqIdx int, ok bool) {
	if len(m.flatList) == 0 {
		return 0, 0, false
	}
	item := m.flatList[m.cursor]
	if item.IsCollection {
		return 0, 0, false
	}
	return item.CollIdx, item.ReqIdx, true
}

// enterBodyEdit switches the request panel to body edit mode.
func (m *Model) enterBodyEdit() tea.Cmd {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return nil
	}
	taH := m.requestH - 6
	if taH < 2 {
		taH = 2
	}
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.SetWidth(m.detailW - 4)
	ta.SetHeight(taH)
	ta.SetValue(m.collections[collIdx].Requests[reqIdx].Body)
	cmd := ta.Focus()
	m.bodyTA = ta
	m.editMode = ui.EditModeBody
	m.focus = FocusDetail
	return cmd
}

// enterHeaderList switches to the header key-value browser.
func (m *Model) enterHeaderList() {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return
	}
	src := m.collections[collIdx].Requests[reqIdx].Headers
	m.pendingHeaders = make(map[string]string, len(src))
	for k, v := range src {
		m.pendingHeaders[k] = v
	}
	m.rebuildHdrKeys()
	m.headerCursor = 0
	m.editMode = ui.EditModeHeaderList
	m.focus = FocusDetail
}

// rebuildHdrKeys refreshes the sorted-keys cache from pendingHeaders.
func (m *Model) rebuildHdrKeys() {
	keys := make([]string, 0, len(m.pendingHeaders))
	for k := range m.pendingHeaders {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m.pendingHdrKeys = keys
}

// enterHeaderValueEdit opens the textinput to edit the currently highlighted header.
func (m *Model) enterHeaderValueEdit() tea.Cmd {
	if len(m.pendingHdrKeys) == 0 {
		return nil
	}
	key := m.pendingHdrKeys[m.headerCursor]
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "value"
	ti.SetValue(m.pendingHeaders[key])
	cmd := ti.Focus()
	m.headerTI = ti
	m.editHeaderKey = key
	m.isNewHeader = false
	m.editMode = ui.EditModeHeaderValue
	return cmd
}

// enterNewHeaderKey opens the textinput to collect a fresh header key.
func (m *Model) enterNewHeaderKey() tea.Cmd {
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "Header-Name"
	cmd := ti.Focus()
	m.headerTI = ti
	m.editHeaderKey = ""
	m.isNewHeader = true
	m.editMode = ui.EditModeNewHeaderKey
	return cmd
}

// confirmHeaderValue commits the textinput value to pendingHeaders.
func (m *Model) confirmHeaderValue() {
	val := strings.TrimSpace(m.headerTI.Value())
	m.headerTI.Blur()
	if m.editHeaderKey != "" {
		m.pendingHeaders[m.editHeaderKey] = val
		m.rebuildHdrKeys()
		// Move cursor to the updated key.
		for i, k := range m.pendingHdrKeys {
			if k == m.editHeaderKey {
				m.headerCursor = i
				break
			}
		}
	}
	m.editMode = ui.EditModeHeaderList
}

// confirmNewHeaderKey validates the key and moves to value entry.
func (m *Model) confirmNewHeaderKey() tea.Cmd {
	key := strings.TrimSpace(m.headerTI.Value())
	m.headerTI.Blur()
	if key == "" {
		m.editMode = ui.EditModeHeaderList
		return nil
	}
	m.editHeaderKey = key
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "value"
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeNewHeaderValue
	return cmd
}

// deleteSelectedHeader removes the highlighted header from pendingHeaders.
func (m *Model) deleteSelectedHeader() {
	if len(m.pendingHdrKeys) == 0 {
		return
	}
	key := m.pendingHdrKeys[m.headerCursor]
	delete(m.pendingHeaders, key)
	m.rebuildHdrKeys()
	if m.headerCursor >= len(m.pendingHdrKeys) && m.headerCursor > 0 {
		m.headerCursor--
	}
}

// saveBody writes the textarea value back to the collection and persists to disk.
func (m *Model) saveBody() {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return
	}
	m.bodyTA.Blur()
	m.collections[collIdx].Requests[reqIdx].Body = m.bodyTA.Value()
	m.bodyTA = textarea.Model{}
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[collIdx], m.collections[collIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "body saved"
		}
	}
}

// saveHeaders commits pendingHeaders (including any in-progress value edit) to
// the collection and persists to disk.
func (m *Model) saveHeaders() {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return
	}

	// Auto-confirm an in-progress value edit before saving.
	if (m.editMode == ui.EditModeHeaderValue || m.editMode == ui.EditModeNewHeaderValue) &&
		m.editHeaderKey != "" {
		m.pendingHeaders[m.editHeaderKey] = strings.TrimSpace(m.headerTI.Value())
		m.rebuildHdrKeys()
		m.headerTI.Blur()
	} else if m.editMode == ui.EditModeNewHeaderKey {
		m.headerTI.Blur() // discard unfinished new key
	}

	m.collections[collIdx].Requests[reqIdx].Headers = m.pendingHeaders
	m.pendingHeaders = nil
	m.pendingHdrKeys = nil
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[collIdx], m.collections[collIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "headers saved"
		}
	}
}

// renderHeaderEditor builds the pre-rendered string shown inside the request
// panel when in any header edit mode.
func (m Model) renderHeaderEditor() string {
	var sb strings.Builder

	switch m.editMode {
	case ui.EditModeHeaderList:
		if len(m.pendingHdrKeys) == 0 {
			sb.WriteString(ui.DimStyle.Render("  (no headers — press 'a' to add one)") + "\n")
		} else {
			for i, k := range m.pendingHdrKeys {
				v := m.pendingHeaders[k]
				if i == m.headerCursor {
					row := fmt.Sprintf("  %-22s %s", k+":", v)
					sb.WriteString(ui.SelectedItemStyle.Width(m.detailW-4).Render(row) + "\n")
				} else {
					sb.WriteString(
						"  " + ui.HeaderKeyStyle.Render(k+":") + " " + ui.HeaderValStyle.Render(v) + "\n",
					)
				}
			}
		}

	case ui.EditModeHeaderValue:
		sb.WriteString(ui.DimStyle.Render(fmt.Sprintf("  Value for %q:", m.editHeaderKey)) + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")

	case ui.EditModeNewHeaderKey:
		sb.WriteString(ui.DimStyle.Render("  New header name:") + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")

	case ui.EditModeNewHeaderValue:
		sb.WriteString(ui.DimStyle.Render(fmt.Sprintf("  Value for %q:", m.editHeaderKey)) + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")
	}

	return sb.String()
}

// ─── New-request wizard helpers ─────────────────────────────────────────────

// targetCollIdx returns the collection index for the currently highlighted item
// (works for both collection headers and request rows).
func (m Model) targetCollIdx() (int, bool) {
	if len(m.flatList) == 0 {
		return 0, false
	}
	return m.flatList[m.cursor].CollIdx, true
}

// enterNewReq kicks off the new-request wizard for the collection at the cursor.
func (m *Model) enterNewReq() tea.Cmd {
	collIdx, ok := m.targetCollIdx()
	if !ok {
		return nil
	}
	m.newReqTargetColl = collIdx
	m.newReqName = ""
	m.newReqMethod = ""
	return m.enterNewReqNameStep()
}

func (m *Model) enterNewReqNameStep() tea.Cmd {
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "My Request"
	if m.newReqName != "" {
		ti.SetValue(m.newReqName)
	}
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeNewReqName
	m.focus = FocusDetail
	return cmd
}

func (m *Model) enterNewReqMethodStep() tea.Cmd {
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "GET"
	if m.newReqMethod != "" {
		ti.SetValue(m.newReqMethod)
	}
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeNewReqMethod
	return cmd
}

func (m *Model) enterNewReqURLStep() tea.Cmd {
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "https://example.com/api"
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeNewReqURL
	return cmd
}

// addNewRequest finalises the wizard and persists the new request to disk.
func (m *Model) addNewRequest(url string) {
	req := collection.Request{
		Name:    m.newReqName,
		Method:  m.newReqMethod,
		URL:     url,
		Headers: map[string]string{},
		Body:    "",
	}
	m.collections[m.newReqTargetColl].Requests = append(
		m.collections[m.newReqTargetColl].Requests, req,
	)
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	m.headerTI.Blur()
	m.editMode = ui.EditModeNone
	m.newReqName = ""
	m.newReqMethod = ""

	if m.newReqTargetColl < len(m.filePaths) {
		if err := collection.Save(m.filePaths[m.newReqTargetColl], m.collections[m.newReqTargetColl]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "request added"
		}
	}
}

// ─── New-collection wizard helpers ──────────────────────────────────────────

// enterNewColl starts the new-collection wizard.
func (m *Model) enterNewColl() tea.Cmd {
	m.newCollName = ""
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "My Collection"
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeNewCollName
	m.focus = FocusDetail
	return cmd
}

// addNewCollection creates a new empty collection JSON file and appends it to
// the in-memory state.
func (m *Model) addNewCollection(name string) {
	m.newCollName = name
	m.headerTI.Blur()
	m.editMode = ui.EditModeNone
	m.newCollName = ""

	filePath := m.uniqueCollectionPath(name)

	newColl := collection.Collection{
		Name:     name,
		Requests: []collection.Request{},
	}

	if err := collection.Save(filePath, newColl); err != nil {
		m.statusMsg = "save error: " + err.Error()
		return
	}

	m.collections = append(m.collections, newColl)
	m.filePaths = append(m.filePaths, filePath)
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	// Move cursor to the new collection header.
	m.cursor = len(m.flatList) - 1
	m.statusMsg = fmt.Sprintf("collection %q created", name)
}

func (m Model) collectionsDirOrDefault() string {
	if m.collectionsDir == "" {
		return "./collections"
	}
	return m.collectionsDir
}

func (m Model) uniqueCollectionPath(name string) string {
	dir := m.collectionsDirOrDefault()
	slug := slugifyCollectionName(name)
	filePath := filepath.Join(dir, slug+".json")

	if _, err := os.Stat(filePath); err == nil {
		for i := 2; ; i++ {
			candidate := filepath.Join(dir, fmt.Sprintf("%s-%d.json", slug, i))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				filePath = candidate
				break
			}
		}
	}
	return filePath
}

func slugifyCollectionName(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		return "collection"
	}
	return slug
}

// renderNewCollEditor builds the editor panel content for the new-collection wizard.
func (m Model) renderNewCollEditor() string {
	var sb strings.Builder
	sb.WriteString(ui.DimStyle.Render("  Collection name:") + "\n\n")
	sb.WriteString("  " + m.headerTI.View() + "\n")
	return sb.String()
}

// ─── Rename / delete collection helpers ─────────────────────────────────────

// enterImportCollection opens a one-line prompt for a collection JSON file.
func (m *Model) enterImportCollection() tea.Cmd {
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = `C:\path\to\collection.json`
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeImportCollPath
	m.focus = FocusDetail
	return cmd
}

// importCollectionFromInput validates and copies a collection JSON into the
// configured collections directory, then adds it to the in-memory list.
func (m *Model) importCollectionFromInput() {
	sourcePath := strings.TrimSpace(m.headerTI.Value())
	if sourcePath == "" {
		return
	}
	m.headerTI.Blur()
	m.editMode = ui.EditModeNone

	coll, err := collection.LoadFile(sourcePath)
	if err != nil {
		m.statusMsg = "import error: " + err.Error()
		return
	}
	if strings.TrimSpace(coll.Name) == "" {
		coll.Name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	if coll.Requests == nil {
		coll.Requests = []collection.Request{}
	}

	targetPath := m.uniqueCollectionPath(coll.Name)
	if err := collection.Save(targetPath, coll); err != nil {
		m.statusMsg = "import error: " + err.Error()
		return
	}

	m.collections = append(m.collections, coll)
	m.filePaths = append(m.filePaths, targetPath)
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	m.cursor = len(m.flatList) - 1
	m.statusMsg = fmt.Sprintf("imported %q", coll.Name)
}

func (m Model) renderImportCollEditor() string {
	var sb strings.Builder
	sb.WriteString(ui.DimStyle.Render("  JSON file to import:") + "\n\n")
	sb.WriteString("  " + m.headerTI.View() + "\n")
	return sb.String()
}

// enterExportCollection opens a prompt for where the selected collection JSON
// should be written.
func (m *Model) enterExportCollection() tea.Cmd {
	if len(m.flatList) == 0 {
		m.statusMsg = "no collection selected"
		return nil
	}
	collIdx := m.flatList[m.cursor].CollIdx
	if collIdx < 0 || collIdx >= len(m.collections) {
		m.statusMsg = "no collection selected"
		return nil
	}
	m.renameCollIdx = collIdx

	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = `exports\collection.json`
	ti.SetValue(filepath.Join("exports", slugifyCollectionName(m.collections[collIdx].Name)+".json"))
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeExportCollPath
	m.focus = FocusDetail
	return cmd
}

func (m *Model) exportCollectionToInput() {
	targetPath := strings.TrimSpace(m.headerTI.Value())
	if targetPath == "" {
		return
	}
	m.headerTI.Blur()
	m.editMode = ui.EditModeNone

	if m.renameCollIdx < 0 || m.renameCollIdx >= len(m.collections) {
		m.statusMsg = "export error: no collection selected"
		return
	}
	if err := collection.Save(targetPath, m.collections[m.renameCollIdx]); err != nil {
		m.statusMsg = "export error: " + err.Error()
		return
	}
	m.statusMsg = "exported to " + targetPath
}

func (m Model) renderExportCollEditor() string {
	var sb strings.Builder
	name := ""
	if m.renameCollIdx >= 0 && m.renameCollIdx < len(m.collections) {
		name = m.collections[m.renameCollIdx].Name
	}
	sb.WriteString(ui.DimStyle.Render(fmt.Sprintf("  Export %q to:", name)) + "\n\n")
	sb.WriteString("  " + m.headerTI.View() + "\n")
	return sb.String()
}

// enterRenameCollection opens a textinput pre-filled with the current collection name.
func (m *Model) enterRenameCollection() tea.Cmd {
	if len(m.flatList) == 0 {
		return nil
	}
	collIdx := m.flatList[m.cursor].CollIdx
	m.renameCollIdx = collIdx

	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "Collection name"
	ti.SetValue(m.collections[collIdx].Name)
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeRenameCollName
	m.focus = FocusDetail
	return cmd
}

// saveCollectionRename commits the new name to the collection and persists to disk.
func (m *Model) saveCollectionRename() {
	name := strings.TrimSpace(m.headerTI.Value())
	if name == "" {
		return
	}
	m.headerTI.Blur()
	m.collections[m.renameCollIdx].Name = name
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	m.editMode = ui.EditModeNone

	if m.renameCollIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[m.renameCollIdx], m.collections[m.renameCollIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("collection renamed to %q", name)
		}
	}
}

// deleteSelectedCollection removes the collection file from disk and from state.
func (m *Model) deleteSelectedCollection() {
	collIdx := m.renameCollIdx
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		_ = os.Remove(m.filePaths[collIdx])
	}

	name := m.collections[collIdx].Name
	m.collections = append(m.collections[:collIdx], m.collections[collIdx+1:]...)
	m.filePaths = append(m.filePaths[:collIdx], m.filePaths[collIdx+1:]...)
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)

	// Keep cursor in bounds.
	if m.cursor >= len(m.flatList) && m.cursor > 0 {
		m.cursor = len(m.flatList) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.statusMsg = fmt.Sprintf("collection %q deleted", name)
}

// renderRenameCollEditor builds the panel content for the rename-collection mode.
func (m Model) renderRenameCollEditor() string {
	var sb strings.Builder
	sb.WriteString(ui.DimStyle.Render("  New collection name:") + "\n\n")
	sb.WriteString("  " + m.headerTI.View() + "\n")
	return sb.String()
}

// renderDeleteCollEditor builds the panel content for the delete-collection confirm mode.
func (m Model) renderDeleteCollEditor() string {
	var sb strings.Builder
	name := ""
	if m.renameCollIdx < len(m.collections) {
		name = m.collections[m.renameCollIdx].Name
	}
	sb.WriteString(ui.ErrorStyle.Render(fmt.Sprintf("  Delete collection %q?", name)) + "\n\n")
	sb.WriteString(ui.DimStyle.Render("  Press y to confirm, any other key to cancel.") + "\n")
	return sb.String()
}

// deleteSelectedRequest removes the highlighted request and persists the change.
func (m *Model) deleteSelectedRequest() {
	if len(m.flatList) == 0 {
		m.editMode = ui.EditModeNone
		return
	}
	item := m.flatList[m.cursor]
	if item.IsCollection {
		m.editMode = ui.EditModeNone
		return
	}
	collIdx, reqIdx := item.CollIdx, item.ReqIdx
	reqs := m.collections[collIdx].Requests
	m.collections[collIdx].Requests = append(reqs[:reqIdx], reqs[reqIdx+1:]...)
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	// Keep cursor in bounds.
	if m.cursor >= len(m.flatList) && m.cursor > 0 {
		m.cursor = len(m.flatList) - 1
	}
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[collIdx], m.collections[collIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "request deleted"
		}
	}
}

// renderNewReqEditor builds the editor panel content for each wizard step.
func (m Model) renderNewReqEditor() string {
	var sb strings.Builder
	switch m.editMode {
	case ui.EditModeNewReqName:
		sb.WriteString(ui.DimStyle.Render("  Request name:") + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")
	case ui.EditModeNewReqMethod:
		sb.WriteString(ui.DimStyle.Render(fmt.Sprintf("  Name: %s", m.newReqName)) + "\n")
		sb.WriteString(ui.DimStyle.Render("  HTTP method (GET POST PUT PATCH DELETE …):") + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")
	case ui.EditModeNewReqURL:
		sb.WriteString(ui.DimStyle.Render(fmt.Sprintf("  Name: %s   Method: %s", m.newReqName, m.newReqMethod)) + "\n")
		sb.WriteString(ui.DimStyle.Render("  URL:") + "\n\n")
		sb.WriteString("  " + m.headerTI.View() + "\n")
	}
	return sb.String()
}

// ─── Method & URL edit helpers ───────────────────────────────────────────────

// enterMethodEdit opens a one-line textinput pre-filled with the current method.
func (m *Model) enterMethodEdit() tea.Cmd {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return nil
	}
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "GET"
	ti.SetValue(m.collections[collIdx].Requests[reqIdx].Method)
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeMethod
	m.focus = FocusDetail
	return cmd
}

// saveMethod commits the textinput value as the new HTTP method and saves to disk.
func (m *Model) saveMethod() {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return
	}
	method := strings.ToUpper(strings.TrimSpace(m.headerTI.Value()))
	if method == "" {
		method = "GET"
	}
	m.headerTI.Blur()
	m.collections[collIdx].Requests[reqIdx].Method = method
	// Rebuild flatList so the sidebar shows the updated method colour immediately.
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[collIdx], m.collections[collIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "method saved"
		}
	}
}

// enterURLEdit opens a one-line textinput pre-filled with the current URL.
func (m *Model) enterURLEdit() tea.Cmd {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return nil
	}
	ti := textinput.New()
	ti.Width = m.detailW - 10
	ti.Prompt = "> "
	ti.Placeholder = "https://example.com"
	ti.SetValue(m.collections[collIdx].Requests[reqIdx].URL)
	cmd := ti.Focus()
	m.headerTI = ti
	m.editMode = ui.EditModeURL
	m.focus = FocusDetail
	return cmd
}

// saveURL commits the textinput value as the new URL and saves to disk.
func (m *Model) saveURL() {
	collIdx, reqIdx, ok := m.editTarget()
	if !ok {
		return
	}
	url := strings.TrimSpace(m.headerTI.Value())
	m.headerTI.Blur()
	m.collections[collIdx].Requests[reqIdx].URL = url
	m.editMode = ui.EditModeNone

	if collIdx < len(m.filePaths) {
		if err := collection.Save(m.filePaths[collIdx], m.collections[collIdx]); err != nil {
			m.statusMsg = "save error: " + err.Error()
		} else {
			m.statusMsg = "url saved"
		}
	}
}

// helpLine returns the bottom status bar padded to the full terminal width
// so there is no visible gap at the bottom of the screen after resize.
func (m Model) helpLine() string {
	var keys string
	switch m.editMode {
	case ui.EditModeBody:
		keys = "  ctrl+s save  esc cancel"
	case ui.EditModeHeaderList:
		keys = "  enter edit  a add  d delete  ctrl+s save  esc cancel"
	case ui.EditModeHeaderValue, ui.EditModeNewHeaderKey, ui.EditModeNewHeaderValue:
		keys = "  enter confirm  esc back  ctrl+s save-all"
	case ui.EditModeNewReqName:
		keys = "  enter next  esc cancel  (step 1/3: name)"
	case ui.EditModeNewReqMethod:
		keys = "  enter next  esc back  (step 2/3: method, blank=GET)"
	case ui.EditModeNewReqURL:
		keys = "  enter add  esc back  (step 3/3: URL)"
	case ui.EditModeNewCollName:
		keys = "  enter create  esc cancel  (collection name)"
	case ui.EditModeRenameCollName:
		keys = "  enter/ctrl+s save  esc cancel  (new collection name)"
	case ui.EditModeImportCollPath:
		keys = "  enter import  esc cancel  (collection json path)"
	case ui.EditModeExportCollPath:
		keys = "  enter export  esc cancel  (target json path)"
	case ui.EditModeDeleteCollConfirm:
		collName := ""
		if m.renameCollIdx < len(m.collections) {
			collName = fmt.Sprintf(" %q", m.collections[m.renameCollIdx].Name)
		}
		keys = fmt.Sprintf("  Delete collection%s?   y = yes   any other key = cancel", collName)
	case ui.EditModeDeleteConfirm:
		req := m.selectedRequest()
		name := "this request"
		if req != nil {
			name = fmt.Sprintf("%q", req.Name)
		}
		keys = fmt.Sprintf("  Delete %s?   y = yes   any other key = cancel", name)
	case ui.EditModeMethod:
		keys = "  enter/ctrl+s save  esc cancel"
	case ui.EditModeURL:
		keys = "  enter/ctrl+s save  esc cancel"
	default:
		if m.viewMode == ViewResponses {
			keys = "  v collections  j/k navigate  enter view  d delete-file  tab focus  f/b scroll  q quit"
		} else {
			keys = "  enter run  n req  C col  i import-json  x export-json  r rename  D del-col  d del-req  m method  u url  e body  h headers  v responses  tab focus  q quit"
		}
	}

	if m.statusMsg != "" {
		keys += "    " + ui.SavedStyle.Render("✓ "+m.statusMsg)
	}

	// Width() pads the line to the full terminal width, filling the background.
	return ui.HelpStyle.Width(m.width).Render(keys)
}

// ─── Private helpers ─────────────────────────────────────────────────────────

// applySize updates width/height and recalculates every derived dimension.
func (m *Model) applySize(w, h int) {
	m.width = w
	m.height = h
	m.recalcLayout()
	if m.response != nil {
		m.responseVP.SetContent(m.formatViewportContent(m.response))
	}
	// Resize live editors to match the new terminal size.
	if m.editMode == ui.EditModeBody {
		taH := m.requestH - 6
		if taH < 2 {
			taH = 2
		}
		m.bodyTA.SetHeight(taH)
		m.bodyTA.SetWidth(m.detailW - 4)
	}
	if m.editMode == ui.EditModeHeaderValue ||
		m.editMode == ui.EditModeNewHeaderKey ||
		m.editMode == ui.EditModeNewHeaderValue ||
		m.editMode == ui.EditModeImportCollPath ||
		m.editMode == ui.EditModeExportCollPath {
		m.headerTI.Width = m.detailW - 10
	}
}

func (m *Model) recalcLayout() {
	if m.width < 20 || m.height < 10 {
		return
	}

	// Sidebar: aim for 28 % of width with a soft minimum of 22.
	// If the terminal is too narrow to honour the minimum without
	// squashing the detail panel, fall back to the raw percentage.
	m.sidebarW = m.width * 28 / 100
	if m.sidebarW < 22 && m.width >= 44 {
		m.sidebarW = 22
	}
	m.detailW = m.width - m.sidebarW
	if m.detailW < 10 {
		m.detailW = 10
	}

	m.totalH = m.height - 1 // reserve 1 row for the help bar

	m.requestH = m.totalH * 35 / 100
	if m.requestH < 6 {
		m.requestH = 6
	}
	m.responseH = m.totalH - m.requestH
	if m.responseH < 6 {
		m.responseH = 6
	}

	// Viewport sits inside the response sub-panel.
	//
	// Fixed rows inside the response panel (innerH = responseH-2):
	//   title(1) + sep(1) + status(1) + blank(1) = 4 above the viewport
	//   blank(1) + scroll%(1)                    = 2 below the viewport
	//   Total fixed = 6  →  vpH = innerH - 6 = responseH - 8
	vpH := m.responseH - 8
	if vpH < 1 {
		vpH = 1
	}
	vpW := m.detailW - 2
	if vpW < 10 {
		vpW = 10
	}
	m.responseVP.Width = vpW
	m.responseVP.Height = vpH

	fileVPHeight := m.totalH - 5
	if fileVPHeight < 1 {
		fileVPHeight = 1
	}
	m.responseFileVP.Width = vpW
	m.responseFileVP.Height = fileVPHeight
}

func (m *Model) enterResponsesView() {
	m.viewMode = ViewResponses
	m.focus = FocusSidebar
	m.refreshResponseFiles()
	m.loadSelectedResponseFile()
}

func (m *Model) responsesDirOrDefault() string {
	if m.responsesDir == "" {
		return "./responses"
	}
	return m.responsesDir
}

func (m *Model) refreshResponseFiles() {
	files, err := responses.List(m.responsesDirOrDefault())
	if err != nil {
		m.responseFiles = nil
		m.responseCursor = 0
		m.selectedResponsePath = ""
		m.selectedResponseContent = ""
		m.responseFileVP.SetContent("")
		m.statusMsg = "responses error: " + err.Error()
		return
	}

	m.responseFiles = files
	if m.responseCursor >= len(m.responseFiles) && m.responseCursor > 0 {
		m.responseCursor = len(m.responseFiles) - 1
	}
	if m.responseCursor < 0 {
		m.responseCursor = 0
	}
}

func (m *Model) moveResponseCursor(delta int) {
	if len(m.responseFiles) == 0 {
		return
	}
	next := m.responseCursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.responseFiles) {
		next = len(m.responseFiles) - 1
	}
	if next == m.responseCursor {
		return
	}
	m.responseCursor = next
	m.loadSelectedResponseFile()
}

func (m *Model) loadSelectedResponseFile() {
	if len(m.responseFiles) == 0 {
		m.selectedResponsePath = ""
		m.selectedResponseContent = ""
		m.responseFileVP.SetContent("")
		return
	}
	file := m.responseFiles[m.responseCursor]
	content, err := responses.Read(file.Path)
	if err != nil {
		m.selectedResponsePath = file.Path
		m.selectedResponseContent = ""
		m.responseFileVP.SetContent("")
		m.statusMsg = "read error: " + err.Error()
		return
	}
	m.selectedResponsePath = file.Path
	m.selectedResponseContent = content
	m.responseFileVP.SetContent(content)
	m.responseFileVP.GotoTop()
}

func (m *Model) deleteSelectedResponseFile() {
	if len(m.responseFiles) == 0 {
		m.statusMsg = "no response files to delete"
		return
	}

	file := m.responseFiles[m.responseCursor]
	if err := responses.Delete(file.Path); err != nil {
		m.statusMsg = "delete error: " + err.Error()
		return
	}

	m.statusMsg = "deleted " + file.Name
	m.refreshResponseFiles()
	m.loadSelectedResponseFile()
}

func (m Model) selectedRequest() *collection.Request {
	if len(m.flatList) == 0 {
		return nil
	}
	item := m.flatList[m.cursor]
	if item.IsCollection {
		return nil
	}
	if item.CollIdx >= len(m.collections) {
		return nil
	}
	reqs := m.collections[item.CollIdx].Requests
	if item.ReqIdx >= len(reqs) {
		return nil
	}
	r := reqs[item.ReqIdx]
	return &r
}

func (m *Model) toggleCollapse() {
	if len(m.flatList) == 0 {
		return
	}
	item := m.flatList[m.cursor]
	if !item.IsCollection {
		return
	}
	m.collapsed[item.CollIdx] = !m.collapsed[item.CollIdx]
	m.flatList = ui.BuildFlatList(m.collections, m.collapsed)
	if m.cursor >= len(m.flatList) {
		m.cursor = len(m.flatList) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) formatViewportContent(resp *httpclient.Response) string {
	var sb strings.Builder
	if len(resp.Headers) > 0 {
		sb.WriteString("Headers:\n")
		keys := make([]string, 0, len(resp.Headers))
		for k := range resp.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vals := strings.Join(resp.Headers[k], ", ")
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, vals))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Body:\n")
	sb.WriteString(resp.Body)
	return sb.String()
}
