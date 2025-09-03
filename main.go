package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"os/exec"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowser mode = iota
	modeMenu
	modeEdit
	modeTimer
	modeExport
)

type vimMode int

const (
	vimNormal vimMode = iota
	vimInsert
	vimVisual
	vimCommand
)

type viewMode int

const (
	viewSplitPane viewMode = iota
	viewEditorOnly
	viewPreviewOnly
)

type blockType string

const (
	blockText     blockType = "text"
	blockMath     blockType = "math"
	blockHeading  blockType = "heading"
	blockCode     blockType = "code"
	blockQuote    blockType = "quote"
	blockList     blockType = "list"
	blockRawLaTeX blockType = "rawlatex"
)

type exportFormat int

const (
	exportPDF exportFormat = iota
	exportHTML
	exportUnicode
	exportMarkdown
)

type tickMsg time.Time

type ContentBlock struct {
	ID         string    `json:"id"`
	Type       blockType `json:"type"`
	Content    string    `json:"content"`
	Rendered   string    `json:"rendered,omitempty"`
	Numbered   bool      `json:"numbered,omitempty"`
	Language   string    `json:"language,omitempty"`
	Level      int       `json:"level,omitempty"`
}

type Template struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Content     []ContentBlock `json:"content"`
	Variables   map[string]string `json:"variables"`
}

type OathDocument struct {
	Version   string            `json:"version"`
	Template  string            `json:"template"`
	Content   []ContentBlock    `json:"content"`
	Variables map[string]string `json:"variables"`
	Created   time.Time         `json:"created"`
	Modified  time.Time         `json:"modified"`
}

type Diagnostic struct {
	Line     int
	Column   int
	Message  string
	Severity string
}

type Completion struct {
	Label      string
	Detail     string
	InsertText string
	Kind       string
	Example    string
}

type RenderedBlock struct {
	Unicode      string
	Errors       []Diagnostic
	LastModified time.Time
}

type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	order    *list.List
}

type cacheItem struct {
	key   string
	value RenderedBlock
}

type renderModel struct {
	cache       *LRUCache
	mathSymbols map[string]string
	commands    []string
}

type lspModel struct {
	completions      []Completion
	activeCompletion int
	showCompletions  bool
	triggerPrefix    string
	diagnostics      []Diagnostic
	symbols          map[string]Completion
}

type FileInfo struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type browserModel struct {
	currentPath string
	files       []FileInfo
	selected    int
	showHidden  bool
	errorMsg    string
}

type vimState struct {
	mode         vimMode
	enabled      bool
	repeatCount  int
	lastCommand  string
	register     string
	registers    map[string]string
	searchTerm   string
	visualStart  int
	visualEnd    int
	cursorPos    int
	yankBuffer   string
}

type documentModel struct {
	blocks       []ContentBlock
	currentBlock int
	editor       textarea.Model
	filepath     string
	modified     bool
	viewMode     viewMode
	splitRatio   float64
	renderer     *renderModel
	lsp          *lspModel
	vim          *vimState
	needsRefresh bool
}

type menuModel struct {
	templates []Template
	selected  int
	input     textinput.Model
}

type exportModel struct {
	formats  []string
	selected int
	filename string
	input    textinput.Model
}

type UserPreferences struct {
	Theme         string  `json:"theme"`
	LastDirectory string  `json:"lastDirectory"`
	SplitRatio    float64 `json:"splitRatio"`
	ViewMode      int     `json:"viewMode"`
	ShowHidden    bool    `json:"showHidden"`
	VimMode       bool    `json:"vimMode"`
}

type model struct {
	mode          mode
	width, height int

	browser  browserModel
	document documentModel
	menu     menuModel
	export   exportModel

	duration  time.Duration
	remaining time.Duration
	ticker    *time.Ticker
	paused    bool
	input     textinput.Model
	notes     textarea.Model
	theme     themeModel

	preferences *UserPreferences
}

type Theme struct {
	Name        string
	Primary     lipgloss.AdaptiveColor
	Secondary   lipgloss.AdaptiveColor
	Accent      lipgloss.AdaptiveColor
	Background  lipgloss.AdaptiveColor
	Foreground  lipgloss.AdaptiveColor
	Success     lipgloss.AdaptiveColor
	Warning     lipgloss.AdaptiveColor
	Error       lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor
	Border      lipgloss.AdaptiveColor
}

var themes = map[string]Theme{
	"default": {
		Name:       "Default",
		Primary:    lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"},
		Secondary:  lipgloss.AdaptiveColor{Light: "#656d76", Dark: "#8b949e"},
		Accent:     lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#a5a5ff"},
		Background: lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0d1117"},
		Foreground: lipgloss.AdaptiveColor{Light: "#24292f", Dark: "#c9d1d9"},
		Success:    lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"},
		Warning:    lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"},
		Error:      lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"},
		Muted:      lipgloss.AdaptiveColor{Light: "#656d76", Dark: "#7d8590"},
		Border:     lipgloss.AdaptiveColor{Light: "#d0d7de", Dark: "#30363d"},
	},
	"gruvbox": {
		Name:       "Gruvbox",
		Primary:    lipgloss.AdaptiveColor{Light: "#076678", Dark: "#83a598"},
		Secondary:  lipgloss.AdaptiveColor{Light: "#665c54", Dark: "#a89984"},
		Accent:     lipgloss.AdaptiveColor{Light: "#8f3f71", Dark: "#d3869b"},
		Background: lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#282828"},
		Foreground: lipgloss.AdaptiveColor{Light: "#3c3836", Dark: "#ebdbb2"},
		Success:    lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"},
		Warning:    lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#fabd2f"},
		Error:      lipgloss.AdaptiveColor{Light: "#cc241d", Dark: "#fb4934"},
		Muted:      lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#928374"},
		Border:     lipgloss.AdaptiveColor{Light: "#bdae93", Dark: "#504945"},
	},
	"nord": {
		Name:       "Nord",
		Primary:    lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#81a1c1"},
		Secondary:  lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#d8dee9"},
		Accent:     lipgloss.AdaptiveColor{Light: "#b48ead", Dark: "#b48ead"},
		Background: lipgloss.AdaptiveColor{Light: "#eceff4", Dark: "#2e3440"},
		Foreground: lipgloss.AdaptiveColor{Light: "#2e3440", Dark: "#eceff4"},
		Success:    lipgloss.AdaptiveColor{Light: "#a3be8c", Dark: "#a3be8c"},
		Warning:    lipgloss.AdaptiveColor{Light: "#ebcb8b", Dark: "#ebcb8b"},
		Error:      lipgloss.AdaptiveColor{Light: "#bf616a", Dark: "#bf616a"},
		Muted:      lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#4c566a"},
		Border:     lipgloss.AdaptiveColor{Light: "#d8dee9", Dark: "#3b4252"},
	},
	"dracula": {
		Name:       "Dracula",
		Primary:    lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#bd93f9"},
		Secondary:  lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#6272a4"},
		Accent:     lipgloss.AdaptiveColor{Light: "#ff79c6", Dark: "#ff79c6"},
		Background: lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#282a36"},
		Foreground: lipgloss.AdaptiveColor{Light: "#282a36", Dark: "#f8f8f2"},
		Success:    lipgloss.AdaptiveColor{Light: "#50fa7b", Dark: "#50fa7b"},
		Warning:    lipgloss.AdaptiveColor{Light: "#f1fa8c", Dark: "#f1fa8c"},
		Error:      lipgloss.AdaptiveColor{Light: "#ff5555", Dark: "#ff5555"},
		Muted:      lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#6272a4"},
		Border:     lipgloss.AdaptiveColor{Light: "#44475a", Dark: "#44475a"},
	},
}

type themeModel struct {
	currentTheme string
	available    []string
	selected     int
}

func newLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *LRUCache) Get(key string) (RenderedBlock, bool) {
	if elem, exists := c.cache[key]; exists {
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheItem).value, true
	}
	return RenderedBlock{}, false
}

func (c *LRUCache) Put(key string, value RenderedBlock) {
	if elem, exists := c.cache[key]; exists {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheItem).value = value
		return
	}

	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.cache, oldest.Value.(*cacheItem).key)
		}
	}

	item := &cacheItem{key, value}
	elem := c.order.PushFront(item)
	c.cache[key] = elem
}

func newRenderModel() *renderModel {
	mathSymbols := map[string]string{
		"\\alpha":   "α",
		"\\beta":    "β",
		"\\gamma":   "γ",
		"\\delta":   "δ",
		"\\epsilon": "ε",
		"\\theta":   "θ",
		"\\lambda":  "λ",
		"\\mu":      "μ",
		"\\pi":      "π",
		"\\sigma":   "σ",
		"\\phi":     "φ",
		"\\omega":   "ω",
		"\\int":     "∫",
		"\\sum":     "∑",
		"\\prod":    "∏",
		"\\sqrt":    "√",
		"\\infty":   "∞",
		"\\partial": "∂",
		"\\nabla":   "∇",
		"\\pm":      "±",
		"\\times":   "×",
		"\\div":     "÷",
		"\\le":      "≤",
		"\\ge":      "≥",
		"\\ne":      "≠",
		"\\approx":  "≈",
		"\\subset":  "⊂",
		"\\supset":  "⊃",
		"\\in":      "∈",
		"\\notin":   "∉",
		"\\cup":     "∪",
		"\\cap":     "∩",
		"\\forall":  "∀",
		"\\exists":  "∃",
	}

	commands := []string{
		"\\alpha", "\\beta", "\\gamma", "\\delta", "\\epsilon", "\\theta",
		"\\lambda", "\\mu", "\\pi", "\\sigma", "\\phi", "\\omega",
		"\\int", "\\sum", "\\prod", "\\sqrt", "\\frac", "\\partial",
		"\\nabla", "\\infty", "\\pm", "\\times", "\\div", "\\le", "\\ge",
		"\\ne", "\\approx", "\\subset", "\\supset", "\\in", "\\notin",
		"\\cup", "\\cap", "\\forall", "\\exists", "\\begin", "\\end",
		"\\textbf", "\\textit", "\\emph", "\\href", "\\url",
	}

	return &renderModel{
		cache:       newLRUCache(50),
		mathSymbols: mathSymbols,
		commands:    commands,
	}
}

func newLSPModel() *lspModel {
	symbols := map[string]Completion{
		"\\alpha": {
			Label:      "\\alpha",
			Detail:     "Greek letter alpha",
			InsertText: "\\alpha",
			Kind:       "symbol",
			Example:    "\\alpha + \\beta = \\gamma",
		},
		"\\beta": {
			Label:      "\\beta",
			Detail:     "Greek letter beta",
			InsertText: "\\beta",
			Kind:       "symbol",
			Example:    "\\beta^2 = 4",
		},
		"\\frac": {
			Label:      "\\frac",
			Detail:     "Fraction",
			InsertText: "\\frac{numerator}{denominator}",
			Kind:       "function",
			Example:    "\\frac{1}{2} + \\frac{3}{4}",
		},
		"\\textbf": {
			Label:      "\\textbf",
			Detail:     "Bold text",
			InsertText: "\\textbf{text}",
			Kind:       "format",
			Example:    "\\textbf{Important note}",
		},
		"\\href": {
			Label:      "\\href",
			Detail:     "Hyperlink",
			InsertText: "\\href{url}{text}",
			Kind:       "link",
			Example:    "\\href{https://example.com}{Example}",
		},
		"\\url": {
			Label:      "\\url",
			Detail:     "URL link",
			InsertText: "\\url{url}",
			Kind:       "link",
			Example:    "\\url{https://example.com}",
		},
	}

	return &lspModel{
		completions:      []Completion{},
		activeCompletion: 0,
		showCompletions:  false,
		diagnostics:      []Diagnostic{},
		symbols:          symbols,
	}
}

func newVimState() *vimState {
	return &vimState{
		mode:        vimNormal,
		enabled:     false,
		repeatCount: 0,
		registers:   make(map[string]string),
		register:    "\"",
	}
}

func (r *renderModel) renderLaTeX(content string) RenderedBlock {
	cacheKey := content + fmt.Sprintf("%d", time.Now().Truncate(time.Minute).Unix())
	
	if cached, exists := r.cache.Get(cacheKey); exists {
		return cached
	}

	rendered := content
	diagnostics := []Diagnostic{}

	for latex, unicode := range r.mathSymbols {
		rendered = strings.ReplaceAll(rendered, latex, unicode)
	}

	rendered = r.handleScripts(rendered)
	rendered = r.handleFormatting(rendered)
	diagnostics = append(diagnostics, r.validateSyntax(content)...)

	result := RenderedBlock{
		Unicode:      rendered,
		Errors:       diagnostics,
		LastModified: time.Now(),
	}

	r.cache.Put(cacheKey, result)
	return result
}

func (r *renderModel) handleScripts(content string) string {
	subscripts := map[string]string{
		"_0": "₀", "_1": "₁", "_2": "₂", "_3": "₃", "_4": "₄",
		"_5": "₅", "_6": "₆", "_7": "₇", "_8": "₈", "_9": "₉",
		"_a": "ₐ", "_e": "ₑ", "_i": "ᵢ", "_o": "ₒ", "_u": "ᵤ",
		"_x": "ₓ", "_y": "ᵧ",
	}

	superscripts := map[string]string{
		"^0": "⁰", "^1": "¹", "^2": "²", "^3": "³", "^4": "⁴",
		"^5": "⁵", "^6": "⁶", "^7": "⁷", "^8": "⁸", "^9": "⁹",
		"^n": "ⁿ",
	}

	result := content
	for latex, unicode := range subscripts {
		result = strings.ReplaceAll(result, latex, unicode)
	}
	for latex, unicode := range superscripts {
		result = strings.ReplaceAll(result, latex, unicode)
	}
	return result
}

func (r *renderModel) handleFormatting(content string) string {
	result := content
	
	result = strings.ReplaceAll(result, "\\textbf{", "**")
	result = strings.ReplaceAll(result, "\\textit{", "*")
	result = strings.ReplaceAll(result, "\\emph{", "*")
	
	braceCount := 0
	var processed strings.Builder
	for i, char := range result {
		if char == '{' && i > 0 {
			braceCount++
		} else if char == '}' && braceCount > 0 {
			braceCount--
			if braceCount == 0 {
				processed.WriteRune('*')
				if i > 0 && result[i-1] == '*' {
					processed.WriteRune('*')
				}
				continue
			}
		}
		processed.WriteRune(char)
	}
	
	return processed.String()
}

func (r *renderModel) validateSyntax(content string) []Diagnostic {
	var diagnostics []Diagnostic
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		braceCount := 0
		for i, char := range line {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount < 0 {
					diagnostics = append(diagnostics, Diagnostic{
						Line:     lineNum + 1,
						Column:   i + 1,
						Message:  "Unmatched closing brace",
						Severity: "error",
					})
				}
			}
		}
		if braceCount > 0 {
			diagnostics = append(diagnostics, Diagnostic{
				Line:     lineNum + 1,
				Column:   len(line),
				Message:  "Unmatched opening brace",
				Severity: "error",
			})
		}

		dollarCount := strings.Count(line, "$")
		if dollarCount%2 != 0 {
			diagnostics = append(diagnostics, Diagnostic{
				Line:     lineNum + 1,
				Column:   strings.LastIndex(line, "$") + 1,
				Message:  "Unmatched math delimiter",
				Severity: "error",
			})
		}
	}

	return diagnostics
}

func (l *lspModel) getCompletions(content string) []Completion {
	var completions []Completion

	words := strings.Fields(content)
	if len(words) == 0 {
		return completions
	}

	currentWord := ""
	for _, word := range words {
		if strings.HasPrefix(word, "\\") {
			currentWord = word
		}
	}

	if currentWord == "" || !strings.HasPrefix(currentWord, "\\") {
		return completions
	}

	for cmd, completion := range l.symbols {
		if strings.HasPrefix(cmd, currentWord) {
			completions = append(completions, completion)
		}
	}

	return completions
}

// func (v *vimState) handleVimInput(key string, editor *textarea.Model) bool {
// 	if !v.enabled {
// 		return false
// 	}
//
// 	switch v.mode {
// 	case vimNormal:
// 		return v.handleNormalMode(key, editor)
// 	case vimInsert:
// 		return v.handleInsertMode(key, editor)
// 	case vimVisual:
// 		return v.handleVisualMode(key, editor)
// 	case vimCommand:
// 		return v.handleCommandMode(key, editor)
// 	}
// 	return false
// }
//
// func (v *vimState) handleNormalMode(key string, editor *textarea.Model) bool {
// 	count := v.getRepeatCount()
//
// 	switch key {
// 	case "i":
// 		v.mode = vimInsert
// 		v.resetRepeatCount()
// 		return true
// 	case "I":
// 		v.mode = vimInsert
// 		editor.CursorStart()
// 		v.resetRepeatCount()
// 		return true
// 	case "a":
// 		v.mode = vimInsert
// 		v.resetRepeatCount()
// 		return true
// 	case "A":
// 		v.mode = vimInsert
// 		editor.CursorEnd()
// 		v.resetRepeatCount()
// 		return true
// 	case "h", "left":
// 		for i := 0; i < count; i++ {
// 			v.moveCursorLeft(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "j", "down":
// 		for i := 0; i < count; i++ {
// 			v.moveCursorDown(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "k", "up":
// 		for i := 0; i < count; i++ {
// 			v.moveCursorUp(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "l", "right":
// 		for i := 0; i < count; i++ {
// 			v.moveCursorRight(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "w":
// 		for i := 0; i < count; i++ {
// 			v.moveWordForward(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "b":
// 		for i := 0; i < count; i++ {
// 			v.moveWordBackward(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "0":
// 		if v.repeatCount == 0 {
// 			editor.CursorStart()
// 			return true
// 		}
// 		v.addToRepeatCount(0)
// 		return true
// 	case "$":
// 		editor.CursorEnd()
// 		v.resetRepeatCount()
// 		return true
// 	case "g":
// 		v.lastCommand = "g"
// 		return true
// 	case "G":
// 		if count > 0 {
// 			v.gotoLine(editor, count)
// 		} else {
// 			editor.CursorEnd()
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "x":
// 		for i := 0; i < count; i++ {
// 			v.deleteChar(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "d":
// 		if v.lastCommand == "d" {
// 			v.deleteLine(editor)
// 			v.lastCommand = ""
// 		} else {
// 			v.lastCommand = "d"
// 		}
// 		return true
// 	case "y":
// 		if v.lastCommand == "y" {
// 			v.yankLine(editor)
// 			v.lastCommand = ""
// 		} else {
// 			v.lastCommand = "y"
// 		}
// 		return true
// 	case "p":
// 		for i := 0; i < count; i++ {
// 			v.paste(editor)
// 		}
// 		v.resetRepeatCount()
// 		return true
// 	case "v":
// 		v.mode = vimVisual
// 		v.visualStart = v.getCurrentPosition(editor)
// 		v.visualEnd = v.visualStart
// 		v.resetRepeatCount()
// 		return true
// 	case ":":
// 		v.mode = vimCommand
// 		v.resetRepeatCount()
// 		return true
// 	case "/":
// 		v.mode = vimCommand
// 		v.resetRepeatCount()
// 		return true
// 	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
// 		num, _ := strconv.Atoi(key)
// 		v.addToRepeatCount(num)
// 		return true
// 	case "esc":
// 		v.lastCommand = ""
// 		v.resetRepeatCount()
// 		return true
// 	}
//
// 	if v.lastCommand == "g" {
// 		switch key {
// 		case "g":
// 			if count > 0 {
// 				v.gotoLine(editor, count)
// 			} else {
// 				editor.CursorStart()
// 			}
// 			v.lastCommand = ""
// 			v.resetRepeatCount()
// 			return true
// 		}
// 		v.lastCommand = ""
// 	}
//
// 	return false
// }
//
// func (v *vimState) handleInsertMode(key string, editor *textarea.Model) bool {
// 	if key == "esc" {
// 		v.mode = vimNormal
// 		return true
// 	}
// 	return false
// }
//
// func (v *vimState) handleVisualMode(key string, editor *textarea.Model) bool {
// 	switch key {
// 	case "esc":
// 		v.mode = vimNormal
// 		return true
// 	case "d":
// 		v.deleteVisualSelection(editor)
// 		v.mode = vimNormal
// 		return true
// 	case "y":
// 		v.yankVisualSelection(editor)
// 		v.mode = vimNormal
// 		return true
// 	case "h", "j", "k", "l", "left", "down", "up", "right":
// 		v.handleNormalMode(key, editor)
// 		v.visualEnd = v.getCurrentPosition(editor)
// 		return true
// 	}
// 	return false
// }
//
// func (v *vimState) handleCommandMode(key string, editor *textarea.Model) bool {
// 	if key == "esc" {
// 		v.mode = vimNormal
// 		return true
// 	}
// 	return false
// }
//
// func (v *vimState) getRepeatCount() int {
// 	if v.repeatCount == 0 {
// 		return 1
// 	}
// 	return v.repeatCount
// }
//
// func (v *vimState) addToRepeatCount(digit int) {
// 	v.repeatCount = v.repeatCount*10 + digit
// }
//
// func (v *vimState) resetRepeatCount() {
// 	v.repeatCount = 0
// }
//
// // Simple helper to estimate current position - not precise but functional
// func (v *vimState) getCurrentPosition(editor *textarea.Model) int {
// 	return len(editor.Value()) / 2 // Rough estimate for cursor position
// }
//
// func (v *vimState) moveWordForward(editor *textarea.Model) {
// 	// Simple implementation: move cursor right several positions
// 	for i := 0; i < 5; i++ {
// 		v.moveCursorRight(editor)
// 	}
// }
//
// func (v *vimState) moveWordBackward(editor *textarea.Model) {
// 	// Simple implementation: move cursor left several positions  
// 	for i := 0; i < 5; i++ {
// 		v.moveCursorLeft(editor)
// 	}
// }
//
// func (v *vimState) deleteChar(editor *textarea.Model) {
// 	content := editor.Value()
// 	if len(content) > 0 {
// 		// Get current content and remove one character
// 		lines := strings.Split(content, "\n")
// 		if len(lines) > 0 && len(lines[0]) > 0 {
// 			newFirstLine := lines[0][:len(lines[0])-1]
// 			lines[0] = newFirstLine
// 			editor.SetValue(strings.Join(lines, "\n"))
// 		}
// 	}
// }
//
// func (v *vimState) deleteLine(editor *textarea.Model) {
// 	content := editor.Value()
// 	lines := strings.Split(content, "\n")
//
// 	if len(lines) > 0 {
// 		v.yankBuffer = lines[0]
// 		if len(lines) > 1 {
// 			newContent := strings.Join(lines[1:], "\n")
// 			editor.SetValue(newContent)
// 		} else {
// 			editor.SetValue("")
// 		}
// 	}
// }
//
// func (v *vimState) yankLine(editor *textarea.Model) {
// 	content := editor.Value()
// 	lines := strings.Split(content, "\n")
//
// 	if len(lines) > 0 {
// 		v.yankBuffer = lines[0]
// 	}
// }
//
// func (v *vimState) paste(editor *textarea.Model) {
// 	if v.yankBuffer == "" {
// 		return
// 	}
//
// 	content := editor.Value()
// 	// Simple paste at end
// 	newContent := content + "\n" + v.yankBuffer
// 	editor.SetValue(newContent)
// }
//
// func (v *vimState) deleteVisualSelection(editor *textarea.Model) {
// 	content := editor.Value()
// 	if len(content) > 0 {
// 		// Simple visual selection delete - remove middle portion
// 		quarter := len(content) / 4
// 		half := len(content) / 2
// 		if quarter < half && half < len(content) {
// 			v.yankBuffer = content[quarter:half]
// 			newContent := content[:quarter] + content[half:]
// 			editor.SetValue(newContent)
// 		}
// 	}
// }
//
// func (v *vimState) yankVisualSelection(editor *textarea.Model) {
// 	content := editor.Value()
// 	if len(content) > 0 {
// 		// Simple visual selection yank - copy middle portion
// 		quarter := len(content) / 4  
// 		half := len(content) / 2
// 		if quarter < half && half < len(content) {
// 			v.yankBuffer = content[quarter:half]
// 		}
// 	}
// }
//
// func (v *vimState) gotoLine(editor *textarea.Model, lineNum int) {
// 	if lineNum == 1 {
// 		editor.CursorStart()
// 	} else {
// 		editor.CursorEnd()
// 	}
// }
//
// func (v *vimState) posToIndex(editor *textarea.Model, pos int) int {
// 	return pos
// }
//
// func (v *vimState) indexToPos(editor *textarea.Model, index int) int {
// 	return index
// }
//
// func (v *vimState) moveCursorLeft(editor *textarea.Model) {
// 	editor.CursorLeft()
// }
//
// func (v *vimState) moveCursorRight(editor *textarea.Model) {
// 	editor.CursorRight()
// }
//
// func (v *vimState) moveCursorUp(editor *textarea.Model) {
// 	editor.CursorUp()
// }
//
// func (v *vimState) moveCursorDown(editor *textarea.Model) {
// 	editor.CursorDown()
// }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func scanDirectory(path string, showHidden bool) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	if path != "/" && path != "." {
		files = append(files, FileInfo{
			Name:  "..",
			Path:  filepath.Dir(path),
			IsDir: true,
		})
	}

	for _, entry := range entries {
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

func getDefaultTemplates() []Template {
	return []Template{
		{
			Name:        "Blank Document",
			Description: "Start with an empty document",
			Content: []ContentBlock{
				{ID: "1", Type: blockHeading, Content: "# Document Title"},
				{ID: "2", Type: blockText, Content: "Start writing here"},
			},
			Variables: make(map[string]string),
		},
		{
			Name:        "Academic Notes",
			Description: "Template for mathematical notes and proofs",
			Content: []ContentBlock{
				{ID: "1", Type: blockHeading, Content: "# Course Notes"},
				{ID: "2", Type: blockHeading, Content: "## Topic"},
				{ID: "3", Type: blockText, Content: "Key concepts:"},
				{ID: "4", Type: blockMath, Content: "$\\int_{a}^{b} f(x) dx = F(b) - F(a)$"},
				{ID: "5", Type: blockText, Content: "Proof:"},
			},
			Variables: make(map[string]string),
		},
		{
			Name:        "Resume",
			Description: "Professional resume template",
			Content: []ContentBlock{
				{ID: "1", Type: blockHeading, Content: "# Your Name"},
				{ID: "2", Type: blockText, Content: "email@example.com | (555) 123-4567"},
				{ID: "3", Type: blockHeading, Content: "## Professional Summary"},
				{ID: "4", Type: blockText, Content: "Brief professional summary"},
				{ID: "5", Type: blockHeading, Content: "## Experience"},
				{ID: "6", Type: blockText, Content: "**Job Title** - Company Name (Year - Year)"},
			},
			Variables: make(map[string]string),
		},
		{
			Name:        "Code Documentation",
			Description: "Template for documenting code projects",
			Content: []ContentBlock{
				{ID: "1", Type: blockHeading, Content: "# Project Name"},
				{ID: "2", Type: blockText, Content: "Brief project description"},
				{ID: "3", Type: blockHeading, Content: "## Installation"},
				{ID: "4", Type: blockCode, Content: "git clone repo\ncd project\nnpm install", Language: "bash"},
				{ID: "5", Type: blockHeading, Content: "## Usage"},
				{ID: "6", Type: blockCode, Content: "const example = require('./example');\nexample.run();", Language: "javascript"},
			},
			Variables: make(map[string]string),
		},
	}
}

func loadUserPreferences() *UserPreferences {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return getDefaultPreferences()
	}

	prefsPath := filepath.Join(homeDir, ".oathkeeper", "preferences.json")
	data, err := ioutil.ReadFile(prefsPath)
	if err != nil {
		return getDefaultPreferences()
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return getDefaultPreferences()
	}

	return &prefs
}

func getDefaultPreferences() *UserPreferences {
	currentDir, _ := os.Getwd()
	return &UserPreferences{
		Theme:         "default",
		LastDirectory: currentDir,
		SplitRatio:    0.5,
		ViewMode:      int(viewSplitPane),
		ShowHidden:    false,
		VimMode:       false,
	}
}

func (m *model) saveUserPreferences() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	prefsDir := filepath.Join(homeDir, ".oathkeeper")
	if err := os.MkdirAll(prefsDir, 0755); err != nil {
		return err
	}

	m.preferences.Theme = m.theme.currentTheme
	m.preferences.LastDirectory = m.browser.currentPath
	m.preferences.SplitRatio = m.document.splitRatio
	m.preferences.ViewMode = int(m.document.viewMode)
	m.preferences.ShowHidden = m.browser.showHidden
	m.preferences.VimMode = m.document.vim.enabled

	data, err := json.MarshalIndent(m.preferences, "", "  ")
	if err != nil {
		return err
	}

	prefsPath := filepath.Join(prefsDir, "preferences.json")
	return ioutil.WriteFile(prefsPath, data, 0644)
}

func initialModel() model {
	prefs := loadUserPreferences()

	ti := textinput.New()
	ti.Placeholder = "e.g., 30m, 1h15m, 90s"
	ti.CharLimit = 20
	ti.Width = 30

	ta := textarea.New()
	ta.Placeholder = "Notes to stay on track"
	ta.SetWidth(50)
	ta.SetHeight(5)

	docEditor := textarea.New()
	docEditor.SetWidth(60)
	docEditor.SetHeight(20)
	docEditor.Placeholder = "Start writing"
	docEditor.Cursor.Style = lipgloss.NewStyle()
	docEditor.Cursor.TextStyle = lipgloss.NewStyle()

	menuInput := textinput.New()
	menuInput.Placeholder = "document-name"
	menuInput.CharLimit = 50
	menuInput.Width = 30

	exportInput := textinput.New()
	exportInput.Placeholder = "output-filename"
	exportInput.CharLimit = 100
	exportInput.Width = 40

	files, _ := scanDirectory(prefs.LastDirectory, prefs.ShowHidden)

	themeNames := make([]string, 0, len(themes))
	for name := range themes {
		themeNames = append(themeNames, name)
	}
	
	return model{
		mode:        modeBrowser,
		input:       ti,
		notes:       ta,
		paused:      false,
		preferences: prefs,
		browser: browserModel{
			currentPath: prefs.LastDirectory,
			files:       files,
			selected:    0,
			showHidden:  prefs.ShowHidden,
		},
		document: documentModel{
			blocks:       []ContentBlock{},
			editor:       docEditor,
			viewMode:     viewMode(prefs.ViewMode),
			splitRatio:   prefs.SplitRatio,
			renderer:     newRenderModel(),
			lsp:          newLSPModel(),
			vim:          newVimState(),
			needsRefresh: false,
		},
		menu: menuModel{
			templates: getDefaultTemplates(),
			selected:  0,
			input:     menuInput,
		},
		export: exportModel{
			formats:  []string{"PDF", "HTML", "Unicode Text", "Markdown"},
			selected: 0,
			input:    exportInput,
		},
		theme: themeModel{
			currentTheme: prefs.Theme,
			available:    themeNames,
			selected:     0,
		},
	}
}

func (m model) Init() tea.Cmd {
	m.document.vim.enabled = m.preferences.VimMode
	return tea.Batch(
		textinput.Blink,
		tea.EnterAltScreen,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeBrowser:
			return m.updateBrowser(msg)
		case modeMenu:
			return m.updateMenu(msg)
		case modeEdit:
			return m.updateEdit(msg)
		case modeTimer:
			return m.updateTimer(msg)
		case modeExport:
			return m.updateExport(msg)
		}

	case tickMsg:
		if m.mode == modeTimer && !m.paused && m.ticker != nil {
			m.remaining -= time.Second
			if m.remaining <= 0 {
				m.ticker.Stop()
				m.ticker = nil
				m.remaining = 0
			} else {
				cmds = append(cmds, waitForTick(m.ticker.C))
			}
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		editorWidth := int(float64(msg.Width) * m.document.splitRatio)
		if editorWidth < 20 {
			editorWidth = 20
		} else if editorWidth > msg.Width-20 {
			editorWidth = msg.Width - 20
		}
		m.document.editor.SetWidth(editorWidth - 4)
		m.document.editor.SetHeight(msg.Height - 8)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.saveUserPreferences()
		return m, tea.Quit
	case "j", "down":
		if m.browser.selected < len(m.browser.files)-1 {
			m.browser.selected++
		}
	case "k", "up":
		if m.browser.selected > 0 {
			m.browser.selected--
		}
	case "h":
		m.browser.showHidden = !m.browser.showHidden
		files, err := scanDirectory(m.browser.currentPath, m.browser.showHidden)
		if err != nil {
			m.browser.errorMsg = err.Error()
		} else {
			m.browser.files = files
			if m.browser.selected >= len(files) {
				m.browser.selected = len(files) - 1
			}
		}
	case "enter":
		if len(m.browser.files) > m.browser.selected {
			selectedFile := m.browser.files[m.browser.selected]
			if selectedFile.IsDir {
				files, err := scanDirectory(selectedFile.Path, m.browser.showHidden)
				if err != nil {
					m.browser.errorMsg = err.Error()
				} else {
					m.browser.currentPath = selectedFile.Path
					m.browser.files = files
					m.browser.selected = 0
					m.browser.errorMsg = ""
				}
			} else if strings.HasSuffix(selectedFile.Name, ".oath") {
				return m.loadDocument(selectedFile.Path)
			}
		}
	case " ":
		m.mode = modeMenu
	}
	return m, nil
}

func (m model) loadDocument(filepath string) (tea.Model, tea.Cmd) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		m.browser.errorMsg = fmt.Sprintf("Error loading file: %v", err)
		return m, nil
	}

	var doc OathDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		m.browser.errorMsg = fmt.Sprintf("Error parsing file: %v", err)
		return m, nil
	}

	m.document.blocks = doc.Content
	m.document.filepath = filepath
	m.document.modified = false
	m.document.currentBlock = 0
	m.document.needsRefresh = true

	if len(m.document.blocks) > 0 {
		m.document.editor.SetValue(m.document.blocks[0].Content)
	}

	m.mode = modeEdit
	return m, textarea.Blink
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.mode = modeBrowser
	case "ctrl+c":
		m.saveUserPreferences()
		return m, tea.Quit
	case "j", "down":
		if m.menu.selected < len(m.menu.templates)-1 {
			m.menu.selected++
		}
	case "k", "up":
		if m.menu.selected > 0 {
			m.menu.selected--
		}
	case "enter":
		template := m.menu.templates[m.menu.selected]
		m.document.blocks = make([]ContentBlock, len(template.Content))
		copy(m.document.blocks, template.Content)
		m.document.currentBlock = 0
		m.document.filepath = ""
		m.document.modified = true
		m.document.needsRefresh = true

		if len(m.document.blocks) > 0 {
			m.document.editor.SetValue(m.document.blocks[0].Content)
		}
		m.mode = modeEdit
		return m, textarea.Blink
	case "t":
		m.mode = modeTimer
		m.input.Focus()
		return m, textinput.Blink
	case "v":
		m.document.vim.enabled = !m.document.vim.enabled
		if m.document.vim.enabled {
			m.document.vim.mode = vimNormal
		}
	}
	return m, nil
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// if m.document.vim.enabled && m.document.editor.Focused() {
	// 	if handled := m.document.vim.handleVimInput(msg.String(), &m.document.editor); handled {
	// 		if len(m.document.blocks) > m.document.currentBlock {
	// 			m.document.blocks[m.document.currentBlock].Content = m.document.editor.Value()
	// 			m.document.modified = true
	// 			m.document.needsRefresh = true
	// 		}
	// 		return m, nil
	// 	}
	//
	// 	if m.document.vim.mode != vimInsert && m.document.vim.mode != vimCommand {
	// 		return m, nil
	// 	}
	// }

	if m.document.lsp.showCompletions {
		switch msg.String() {
		case "j", "down":
			if m.document.lsp.activeCompletion < len(m.document.lsp.completions)-1 {
				m.document.lsp.activeCompletion++
			}
			return m, nil
		case "k", "up":
			if m.document.lsp.activeCompletion > 0 {
				m.document.lsp.activeCompletion--
			}
			return m, nil
		case "enter", "tab":
			if len(m.document.lsp.completions) > m.document.lsp.activeCompletion {
				completion := m.document.lsp.completions[m.document.lsp.activeCompletion]
				currentContent := m.document.editor.Value()

				if strings.Contains(currentContent, m.document.lsp.triggerPrefix) {
					newContent := strings.Replace(currentContent, m.document.lsp.triggerPrefix, completion.InsertText, 1)
					m.document.editor.SetValue(newContent)
				}

				m.document.lsp.showCompletions = false
				m.document.lsp.completions = []Completion{}
			}
			return m, nil
		case "esc":
			m.document.lsp.showCompletions = false
			m.document.lsp.completions = []Completion{}
			return m, nil
		}
	}

	if m.document.editor.Focused() {
		if msg.Type == tea.KeyEsc && !m.document.lsp.showCompletions {
			if len(m.document.blocks) > m.document.currentBlock {
				m.document.blocks[m.document.currentBlock].Content = m.document.editor.Value()
				m.document.modified = true
				m.document.needsRefresh = true

				content := m.document.editor.Value()
				rendered := m.document.renderer.renderLaTeX(content)
				m.document.lsp.diagnostics = rendered.Errors
			}
			m.document.editor.Blur()
			if m.document.vim.enabled {
				m.document.vim.mode = vimNormal
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.document.editor, cmd = m.document.editor.Update(msg)

		content := m.document.editor.Value()
		words := strings.Fields(content)
		if len(words) > 0 {
			lastWord := words[len(words)-1]
			if strings.HasPrefix(lastWord, "\\") && len(lastWord) > 1 {
				completions := m.document.lsp.getCompletions(content)
				if len(completions) > 0 {
					m.document.lsp.completions = completions
					m.document.lsp.showCompletions = true
					m.document.lsp.activeCompletion = 0
					m.document.lsp.triggerPrefix = lastWord
				}
			} else {
				m.document.lsp.showCompletions = false
			}
		}

		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.mode = modeMenu
	case "ctrl+c":
		m.saveUserPreferences()
		return m, tea.Quit
	case "j", "down":
		if m.document.currentBlock < len(m.document.blocks)-1 {
			m.document.currentBlock++
			m.document.editor.SetValue(m.document.blocks[m.document.currentBlock].Content)
		}
	case "k", "up":
		if m.document.currentBlock > 0 {
			m.document.currentBlock--
			m.document.editor.SetValue(m.document.blocks[m.document.currentBlock].Content)
		}
	case "enter":
		if len(m.document.blocks) > m.document.currentBlock {
			m.document.editor.Focus()
			if m.document.vim.enabled {
				m.document.vim.mode = vimNormal
			}
			return m, textarea.Blink
		}
	case "n":
		newBlock := ContentBlock{
			ID:      fmt.Sprintf("%d", len(m.document.blocks)+1),
			Type:    blockText,
			Content: "",
		}
		m.document.blocks = append(m.document.blocks, newBlock)
		m.document.currentBlock = len(m.document.blocks) - 1
		m.document.editor.SetValue("")
		m.document.modified = true
		m.document.needsRefresh = true
		m.document.editor.Focus()
		if m.document.vim.enabled {
			m.document.vim.mode = vimInsert
		}
		return m, textarea.Blink
	case "m":
		if len(m.document.blocks) > m.document.currentBlock {
			m.document.blocks[m.document.currentBlock].Type = blockMath
			m.document.modified = true
			m.document.needsRefresh = true
		}
	case "c":
		if len(m.document.blocks) > m.document.currentBlock {
			m.document.blocks[m.document.currentBlock].Type = blockCode
			m.document.modified = true
			m.document.needsRefresh = true
		}
	case "l":
		if len(m.document.blocks) > m.document.currentBlock {
			m.document.blocks[m.document.currentBlock].Type = blockList
			m.document.modified = true
			m.document.needsRefresh = true
		}
	case "r":
		if len(m.document.blocks) > m.document.currentBlock {
			m.document.blocks[m.document.currentBlock].Type = blockRawLaTeX
			m.document.modified = true
			m.document.needsRefresh = true
		}
	case "s":
		if m.document.filepath == "" || strings.Contains(m.document.filepath, "document.oath") {
			return m, m.saveDocument()
		}
		return m, m.saveDocument()
	case "T":
		m.theme.selected = (m.theme.selected + 1) % len(m.theme.available)
		m.theme.currentTheme = m.theme.available[m.theme.selected]
	case "V":
		m.document.vim.enabled = !m.document.vim.enabled
		if m.document.vim.enabled {
			m.document.vim.mode = vimNormal
		}
	case "e":
		m.mode = modeExport
		m.export.input.Focus()
		return m, textinput.Blink
	case "t":
		m.mode = modeTimer
		m.input.Focus()
		return m, textinput.Blink
	case "1":
		m.document.viewMode = viewEditorOnly
	case "2":
		m.document.viewMode = viewSplitPane
	case "3":
		m.document.viewMode = viewPreviewOnly
	case "=":
		if m.document.splitRatio < 0.8 {
			m.document.splitRatio += 0.1
			editorWidth := int(float64(m.width) * m.document.splitRatio)
			if editorWidth < 20 {
				editorWidth = 20
			}
			m.document.editor.SetWidth(editorWidth - 4)
		}
	case "-":
		if m.document.splitRatio > 0.2 {
			m.document.splitRatio -= 0.1
			editorWidth := int(float64(m.width) * m.document.splitRatio)
			if editorWidth < 20 {
				editorWidth = 20
			}
			m.document.editor.SetWidth(editorWidth - 4)
		}
	case "d":
		if len(m.document.blocks) > 1 && m.document.currentBlock < len(m.document.blocks) {
			m.document.blocks = append(m.document.blocks[:m.document.currentBlock],
				m.document.blocks[m.document.currentBlock+1:]...)
			if m.document.currentBlock >= len(m.document.blocks) {
				m.document.currentBlock = len(m.document.blocks) - 1
			}
			if len(m.document.blocks) > m.document.currentBlock {
				m.document.editor.SetValue(m.document.blocks[m.document.currentBlock].Content)
			}
			m.document.modified = true
			m.document.needsRefresh = true
		}
	case "ctrl+l":
		m.document.needsRefresh = true
	}

	return m, nil
}

func (m model) updateTimer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.notes.Focused() {
		if msg.Type == tea.KeyEsc {
			m.notes.Blur()
			return m, nil
		}
		m.notes, cmd = m.notes.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.mode = modeEdit
	case "ctrl+c":
		m.saveUserPreferences()
		return m, tea.Quit
	case "enter":
		if !m.input.Focused() {
			return m, nil
		}
		d, err := time.ParseDuration(m.input.Value())
		if err == nil && d > 0 {
			m.duration = d
			m.remaining = d
			m.paused = false
			m.ticker = time.NewTicker(time.Second)
			cmds = append(cmds, waitForTick(m.ticker.C))
		}
		return m, tea.Batch(cmds...)
	case "p":
		if m.ticker != nil {
			m.paused = true
			m.ticker.Stop()
			m.ticker = nil
		}
	case "r":
		if m.paused && m.remaining > 0 {
			m.paused = false
			m.ticker = time.NewTicker(time.Second)
			cmds = append(cmds, waitForTick(m.ticker.C))
		}
	case "w":
		if m.ticker != nil {
			m.ticker.Stop()
			m.ticker = nil
		}
		m.input.Focus()
	case "n":
		m.notes.Focus()
		cmds = append(cmds, textarea.Blink)
	}

	if m.input.Focused() {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.export.input.Focused() {
		if msg.Type == tea.KeyEsc {
			m.export.input.Blur()
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			filename := strings.TrimSpace(m.export.input.Value())
			if filename == "" {
				filename = m.getSmartFilename()
			}
			return m, m.exportDocument(filename, exportFormat(m.export.selected))
		}
		var cmd tea.Cmd
		m.export.input, cmd = m.export.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.mode = modeEdit
	case "ctrl+c":
		m.saveUserPreferences()
		return m, tea.Quit
	case "j", "down":
		if m.export.selected < len(m.export.formats)-1 {
			m.export.selected++
		}
	case "k", "up":
		if m.export.selected > 0 {
			m.export.selected--
		}
	case "enter":
		m.export.input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func waitForTick(c <-chan time.Time) tea.Cmd {
	return func() tea.Msg {
		return tickMsg(<-c)
	}
}

func (m model) getSmartFilename() string {
	for _, block := range m.document.blocks {
		if block.Type == blockHeading && strings.TrimSpace(block.Content) != "" {
			title := strings.TrimSpace(block.Content)
			title = strings.TrimLeft(title, "#")
			title = strings.TrimSpace(title)
			
			if title != "" && title != "Document Title" {
				filename := strings.ToLower(title)
				filename = strings.ReplaceAll(filename, " ", "-")
				var cleanName strings.Builder
				for _, r := range filename {
					if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
						cleanName.WriteRune(r)
					}
				}
				result := cleanName.String()
				result = strings.Trim(result, "-")
				if result != "" {
					return result
				}
			}
		}
	}
	
	if m.document.filepath != "" {
		base := filepath.Base(m.document.filepath)
		return strings.TrimSuffix(base, ".oath")
	}
	
	return "document"
}

func (m model) saveDocument() tea.Cmd {
	return func() tea.Msg {
		doc := OathDocument{
			Version:   "1.0",
			Template:  "custom",
			Content:   m.document.blocks,
			Variables: make(map[string]string),
			Created:   time.Now(),
			Modified:  time.Now(),
		}

		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil
		}

		filename := m.getSmartFilename() + ".oath"
		if m.document.filepath != "" {
			filename = m.document.filepath
		} else {
			filename = filepath.Join(m.browser.currentPath, filename)
		}
		
		err = ioutil.WriteFile(filename, data, 0644)
		if err == nil {
			m.document.filepath = filename
			m.document.modified = false
		}
		return nil
	}
}

func (m model) exportDocument(filename string, format exportFormat) tea.Cmd {
	return func() tea.Msg {
		switch format {
		case exportPDF:
			return m.generatePDF(filename)
		case exportHTML:
			content := m.generateHTML()
			fullPath := filepath.Join(m.browser.currentPath, filename+".html")
			return ioutil.WriteFile(fullPath, []byte(content), 0644)
		case exportUnicode:
			content := m.generateUnicode()
			fullPath := filepath.Join(m.browser.currentPath, filename+".txt")
			return ioutil.WriteFile(fullPath, []byte(content), 0644)
		case exportMarkdown:
			content := m.generateMarkdown()
			fullPath := filepath.Join(m.browser.currentPath, filename+".md")
			return ioutil.WriteFile(fullPath, []byte(content), 0644)
		}
		return nil
	}
}

func (m model) generatePDF(filename string) error {
	latexContent := m.generateLaTeX()
	texPath := filepath.Join(m.browser.currentPath, filename+".tex")
	
	err := ioutil.WriteFile(texPath, []byte(latexContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write LaTeX file: %v", err)
	}
	
	_, err = exec.LookPath("pdflatex")
	if err != nil {
		return fmt.Errorf("pdflatex not found. LaTeX file saved as %s.tex", filename)
	}
	
	oldDir, _ := os.Getwd()
	os.Chdir(m.browser.currentPath)
	defer os.Chdir(oldDir)
	
	cmd := exec.Command("pdflatex", "-interaction=nonstopmode", filename+".tex")
	_, err = cmd.CombinedOutput()
	
	if err != nil {
		return fmt.Errorf("pdflatex failed: %v", err)
	}
	
	files, _ := ioutil.ReadDir(".")
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, filename+".") && 
		   (strings.HasSuffix(name, ".aux") || 
		    strings.HasSuffix(name, ".log") || 
		    strings.HasSuffix(name, ".fls") ||
		    strings.HasSuffix(name, ".fdb_latexmk") ||
		    strings.HasSuffix(name, ".synctex.gz") ||
		    strings.HasSuffix(name, ".out") ||
		    strings.HasSuffix(name, ".toc") ||
		    strings.HasSuffix(name, ".tex")) {
			os.Remove(name)
		}
	}
	
	return nil
}

func (m model) generateLaTeX() string {
	var content strings.Builder
	content.WriteString("\\documentclass{article}\n")
	content.WriteString("\\usepackage{amsmath}\n")
	content.WriteString("\\usepackage{amsfonts}\n")
	content.WriteString("\\usepackage{amssymb}\n")
	content.WriteString("\\usepackage[utf8]{inputenc}\n")
	content.WriteString("\\usepackage{url}\n")
	content.WriteString("\\usepackage{hyperref}\n")
	content.WriteString("\\usepackage{listings}\n")
	content.WriteString("\\usepackage{xcolor}\n")
	content.WriteString("\\lstset{basicstyle=\\ttfamily,breaklines=true}\n")
	content.WriteString("\\begin{document}\n\n")

	for i, block := range m.document.blocks {
		switch block.Type {
		case blockHeading:
			level := strings.Count(strings.TrimSpace(block.Content), "#")
			title := strings.TrimSpace(strings.TrimLeft(block.Content, "#"))
			
			if block.Numbered {
				switch level {
				case 1:
					content.WriteString(fmt.Sprintf("\\section{%s}\n", title))
				case 2:
					content.WriteString(fmt.Sprintf("\\subsection{%s}\n", title))
				case 3:
					content.WriteString(fmt.Sprintf("\\subsubsection{%s}\n", title))
				default:
					content.WriteString(fmt.Sprintf("\\paragraph{%s}\n", title))
				}
			} else {
				switch level {
				case 1:
					content.WriteString(fmt.Sprintf("\\section*{%s}\n", title))
				case 2:
					content.WriteString(fmt.Sprintf("\\subsection*{%s}\n", title))
				case 3:
					content.WriteString(fmt.Sprintf("\\subsubsection*{%s}\n", title))
				default:
					content.WriteString(fmt.Sprintf("\\paragraph*{%s}\n", title))
				}
			}
		case blockMath:
			content.WriteString(processDelimiterBasedMath(block.Content))
		case blockCode:
			language := block.Language
			if language == "" {
				language = "text"
			}
			content.WriteString(fmt.Sprintf("\\begin{lstlisting}[language=%s]\n%s\n\\end{lstlisting}\n", language, block.Content))
		case blockQuote:
			content.WriteString(fmt.Sprintf("\\begin{quote}\n%s\n\\end{quote}\n", block.Content))
		case blockList:
			content.WriteString("\\begin{itemize}\n")
			lines := strings.Split(block.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
					item := strings.TrimSpace(line[2:])
					content.WriteString(fmt.Sprintf("\\item %s\n", item))
				}
			}
			content.WriteString("\\end{itemize}\n")
		case blockRawLaTeX:
			content.WriteString(block.Content)
			content.WriteString("\n")
		default:
			text := block.Content
			text = convertInlineMath(text)
			text = smartFormatText(text)
			
			if strings.Contains(text, "http") {
				words := strings.Fields(text)
				for j, word := range words {
					if strings.HasPrefix(word, "http") {
						words[j] = "\\url{" + word + "}"
					}
				}
				text = strings.Join(words, " ")
			}
			
			content.WriteString(text)
			content.WriteString("\n")
		}
		
		if i < len(m.document.blocks)-1 {
			content.WriteString("\\vspace{0.8em}\n\n")
		}
	}

	content.WriteString("\\end{document}\n")
	return content.String()
}

func convertInlineMath(text string) string {
	result := strings.Builder{}
	inMath := false
	
	for i, char := range text {
		if char == '$' {
			if i > 0 && text[i-1] == '\\' {
				result.WriteRune(char)
				continue
			}
			
			if !inMath {
				result.WriteString("\\(")
				inMath = true
			} else {
				result.WriteString("\\)")
				inMath = false
			}
		} else {
			result.WriteRune(char)
		}
	}
	
	return result.String()
}

func smartFormatText(text string) string {
	result := strings.Builder{}
	inMath := false
	i := 0
	
	for i < len(text) {
		if i < len(text)-1 && text[i:i+2] == "\\(" {
			result.WriteString("\\(")
			inMath = true
			i += 2
			continue
		}
		if i < len(text)-1 && text[i:i+2] == "\\)" {
			result.WriteString("\\)")
			inMath = false
			i += 2
			continue
		}
		
		if inMath {
			result.WriteByte(text[i])
			i++
			continue
		}
		
		if i < len(text)-3 && text[i:i+2] == "**" {
			end := strings.Index(text[i+2:], "**")
			if end != -1 && end > 0 { 
				content := text[i+2 : i+2+end]
				result.WriteString("\\textbf{" + content + "}")
				i += 4 + end
				continue
			}
		}
		
		if text[i] == '*' && (i == 0 || text[i-1] != '*') && (i == len(text)-1 || text[i+1] != '*') {
			end := -1
			for j := i + 1; j < len(text); j++ {
				if text[j] == '*' && (j == len(text)-1 || text[j+1] != '*') && (j == 0 || text[j-1] != '*') {
					end = j
					break
				}
			}
			if end != -1 && end > i+1 { 
				content := text[i+1 : end]
				result.WriteString("\\textit{" + content + "}")
				i = end + 1
				continue
			}
		}
		
		result.WriteByte(text[i])
		i++
	}
	
	return result.String()
}
// maybe parser based system in due time if i ever read this comment again 
func processDelimiterBasedMath(rawContent string) string {
	var result strings.Builder
	content := strings.TrimSpace(rawContent)
	
	result.WriteString("\\vspace{0.5em}\n") 
	
	i := 0
	for i < len(content) {
		if i < len(content)-1 && content[i:i+2] == "$$" {
			end := strings.Index(content[i+2:], "$$")
			if end != -1 {
				mathContent := content[i+2 : i+2+end]
				result.WriteString("\\vspace{0.3em}\n\\begin{equation*}\n" + mathContent + "\n\\end{equation*}\n\\vspace{0.3em}\n")
				i += 4 + end
				continue
			}
		}
		
		if content[i] == '$' {
			end := strings.Index(content[i+1:], "$")
			if end != -1 {
				mathContent := content[i+1 : i+1+end]
				result.WriteString("\\(" + mathContent + "\\)")
				i += 2 + end
				continue
			}
		}
		
		if content[i] == '\n' {
			if i < len(content)-1 && content[i+1] == '\n' {
				result.WriteString("\n\n")
				i += 2
				continue
			} else {
				result.WriteString("\n")
			}
		} else {
			result.WriteByte(content[i])
		}
		i++
	}
	
	result.WriteString("\\vspace{0.5em}\n") 
	return result.String()
}
func escapeLaTeX(text string) string {
	replacements := map[string]string{
		"&":  "\\&",
		"%":  "\\%",
		"#":  "\\#",
		"_":  "\\_",
		"~":  "\\textasciitilde{}",
	}
	
	result := text
	for char, escaped := range replacements {
		result = strings.ReplaceAll(result, "\\"+char, "TEMP_ESCAPE_"+char)
		result = strings.ReplaceAll(result, char, escaped)
		result = strings.ReplaceAll(result, "TEMP_ESCAPE_"+char, "\\"+char)
	}
	
	return result
}

func (m model) generateHTML() string {
	var content strings.Builder
	content.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	content.WriteString("<meta charset=\"UTF-8\">\n")
	content.WriteString("<title>Document</title>\n")
	content.WriteString("<script src=\"https://polyfill.io/v3/polyfill.min.js?features=es6\"></script>\n")
	content.WriteString("<script id=\"MathJax-script\" async src=\"https://cdn.jsdelivr.net/npm/mathjax@3/es5/tex-mml-chtml.js\"></script>\n")
	content.WriteString("<link rel=\"stylesheet\" href=\"https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.8.0/styles/default.min.css\">\n")
	content.WriteString("<script src=\"https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.8.0/highlight.min.js\"></script>\n")
	content.WriteString("<style>\n")
	content.WriteString("body { font-family: serif; max-width: 800px; margin: 0 auto; padding: 2rem; line-height: 1.6; }\n")
	content.WriteString("h1, h2, h3 { color: #333; }\n")
	content.WriteString("code { background-color: #f4f4f4; padding: 2px 4px; border-radius: 3px; }\n")
	content.WriteString("pre { background-color: #f4f4f4; padding: 1rem; border-radius: 5px; overflow-x: auto; }\n")
	content.WriteString("blockquote { border-left: 4px solid #ddd; margin: 0; padding-left: 1rem; font-style: italic; }\n")
	content.WriteString("</style>\n")
	content.WriteString("</head>\n<body>\n")

	for _, block := range m.document.blocks {
		switch block.Type {
		case blockHeading:
			level := strings.Count(strings.TrimSpace(block.Content), "#")
			title := strings.TrimSpace(strings.TrimLeft(block.Content, "#"))
			content.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, title, level))
		case blockMath:
			content.WriteString(fmt.Sprintf("<p>\\[%s\\]</p>\n", strings.Trim(block.Content, "$")))
		case blockCode:
			language := block.Language
			if language == "" {
				language = "text"
			}
			content.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>\n", language, block.Content))
		case blockQuote:
			content.WriteString(fmt.Sprintf("<blockquote>%s</blockquote>\n", block.Content))
		case blockList:
			content.WriteString("<ul>\n")
			lines := strings.Split(block.Content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
					item := strings.TrimSpace(line[2:])
					content.WriteString(fmt.Sprintf("<li>%s</li>\n", item))
				}
			}
			content.WriteString("</ul>\n")
		case blockRawLaTeX:
			content.WriteString(fmt.Sprintf("<div class=\"raw-latex\">\\[%s\\]</div>\n", block.Content))
		default:
			text := block.Content
			text = strings.ReplaceAll(text, "**", "<strong>")
			text = strings.ReplaceAll(text, "**", "</strong>")
			text = strings.ReplaceAll(text, "*", "<em>")
			text = strings.ReplaceAll(text, "*", "</em>")
			
			if strings.Contains(text, "http") {
				text = strings.ReplaceAll(text, "http", "<a href=\"http")
				text = strings.ReplaceAll(text, " ", "\"> ")
			}
			
			content.WriteString(fmt.Sprintf("<p>%s</p>\n", text))
		}
	}

	content.WriteString("<script>hljs.highlightAll();</script>\n")
	content.WriteString("</body>\n</html>\n")
	return content.String()
}

func (m model) generateUnicode() string {
	var content strings.Builder

	for _, block := range m.document.blocks {
		switch block.Type {
		case blockCode:
			content.WriteString("```")
			if block.Language != "" {
				content.WriteString(block.Language)
			}
			content.WriteString("\n")
			content.WriteString(block.Content)
			content.WriteString("\n```\n\n")
		case blockQuote:
			lines := strings.Split(block.Content, "\n")
			for _, line := range lines {
				content.WriteString("> " + line + "\n")
			}
			content.WriteString("\n")
		default:
			rendered := m.document.renderer.renderLaTeX(block.Content)
			content.WriteString(rendered.Unicode)
			content.WriteString("\n\n")
		}
	}

	return content.String()
}

func (m model) generateMarkdown() string {
	var content strings.Builder

	for _, block := range m.document.blocks {
		switch block.Type {
		case blockCode:
			content.WriteString("```")
			if block.Language != "" {
				content.WriteString(block.Language)
			}
			content.WriteString("\n")
			content.WriteString(block.Content)
			content.WriteString("\n```\n\n")
		case blockQuote:
			lines := strings.Split(block.Content, "\n")
			for _, line := range lines {
				content.WriteString("> " + line + "\n")
			}
			content.WriteString("\n")
		case blockList:
			content.WriteString(block.Content)
			content.WriteString("\n\n")
		case blockMath:
			content.WriteString("$")
			content.WriteString(strings.Trim(block.Content, "$"))
			content.WriteString("$\n\n")
		case blockRawLaTeX:
			content.WriteString("```latex\n")
			content.WriteString(block.Content)
			content.WriteString("\n```\n\n")
		default:
			content.WriteString(block.Content)
			content.WriteString("\n\n")
		}
	}

	return content.String()
}

func (m model) View() string {
	switch m.mode {
	case modeBrowser:
		return m.viewBrowser()
	case modeMenu:
		return m.viewMenu()
	case modeEdit:
		return m.viewEdit()
	case modeTimer:
		return m.viewTimer()
	case modeExport:
		return m.viewExport()
	}
	return ""
}

func (m model) viewBrowser() string {
	var content strings.Builder

	theme := m.getCurrentTheme()
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(m.width).
		Align(lipgloss.Center)

	pathStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	selectedStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	dirStyle := lipgloss.NewStyle().
		Foreground(theme.Primary)

	fileStyle := lipgloss.NewStyle().
		Foreground(theme.Foreground)

	oathStyle := lipgloss.NewStyle().
		Foreground(theme.Success).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	errorStyle := lipgloss.NewStyle().
		Foreground(theme.Error)

	content.WriteString(titleStyle.Render("Oathkeeper - File Browser"))
	content.WriteString("\n\n")
	content.WriteString(pathStyle.Render("Current directory: " + m.browser.currentPath))
	content.WriteString("\n\n")

	if m.browser.errorMsg != "" {
		content.WriteString(errorStyle.Render("Error: " + m.browser.errorMsg))
		content.WriteString("\n\n")
	}

	maxVisible := m.height - 8
	start := 0
	end := len(m.browser.files)

	if len(m.browser.files) > maxVisible {
		start = m.browser.selected - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(m.browser.files) {
			end = len(m.browser.files)
			start = end - maxVisible
		}
	}

	for i := start; i < end; i++ {
		file := m.browser.files[i]
		cursor := "  "
		if i == m.browser.selected {
			cursor = "> "
		}

		var style lipgloss.Style
		icon := ""

		if file.IsDir {
			style = dirStyle
			icon = "d "
		} else if strings.HasSuffix(file.Name, ".oath") {
			style = oathStyle
			icon = "o "
		} else {
			style = fileStyle
			icon = "f "
		}

		line := cursor + icon + file.Name
		if i == m.browser.selected {
			line = selectedStyle.Render(line)
		} else {
			line = style.Render(line)
		}

		content.WriteString(line)
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("j/k: navigate | enter: select | space: new document | h: toggle hidden | q: quit"))

	return content.String()
}

func (m model) viewMenu() string {
	var content strings.Builder
	theme := m.getCurrentTheme()  

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(m.width).
		Align(lipgloss.Center)

	selectedStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	vimIndicator := ""
	if m.document.vim.enabled {
		vimIndicator = " (Vim: ON)"
	} else {
		vimIndicator = " (Vim: OFF)"
	}

	content.WriteString(titleStyle.Render("Oathkeeper - Document Templates" + vimIndicator))
	content.WriteString("\n\nSelect a template:\n\n")

	for i, template := range m.menu.templates {
		cursor := "  "
		if i == m.menu.selected {
			cursor = "> "
		}

		line := cursor + template.Name + " - " + template.Description
		if i == m.menu.selected {
			line = selectedStyle.Render(line)
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(helpStyle.Render("j/k: navigate | enter: select | v: toggle vim | t: timer | q: back"))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content.String(),
	)
}

func (m model) viewEdit() string {
	theme := m.getCurrentTheme()
	switch m.document.viewMode {
	case viewEditorOnly:
		return m.renderEditor(m.width, m.height)
	case viewPreviewOnly:
		return m.renderPreview(m.width, m.height)
	case viewSplitPane:
		editorWidth := int(float64(m.width) * m.document.splitRatio)
		previewWidth := m.width - editorWidth - 1

		if editorWidth < 20 {
			editorWidth = 20
			previewWidth = m.width - 21
		} else if previewWidth < 20 {
			previewWidth = 20
			editorWidth = m.width - 21
		}

		editor := m.renderEditor(editorWidth, m.height)
		preview := m.renderPreview(previewWidth, m.height)

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			editor,
			lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(theme.Border).
				Height(m.height).
				Render(preview),
		)
	}
	return ""
}

func (m model) renderEditor(width, height int) string {
	var content strings.Builder
	theme := m.getCurrentTheme()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(width).
		Align(lipgloss.Center)

	blockStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		Width(width - 4)

	currentBlockStyle := blockStyle.Copy().
		BorderForeground(theme.Primary)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Width(width)

	completionStyle := lipgloss.NewStyle().
		Background(theme.Secondary).
		Foreground(theme.Foreground).
		Padding(0, 1)

	selectedCompletionStyle := completionStyle.Copy().
		Background(theme.Primary).
		Foreground(theme.Background)

	modifiedIndicator := ""
	if m.document.modified {
		modifiedIndicator = " *"
	}

	filename := "untitled.oath"
	if m.document.filepath != "" {
		filename = filepath.Base(m.document.filepath)
	} else {
		for _, block := range m.document.blocks {
			if block.Type == blockHeading && strings.TrimSpace(block.Content) != "" {
				title := strings.TrimSpace(block.Content)
				title = strings.TrimLeft(title, "#")
				title = strings.TrimSpace(title)
				if title != "" && title != "Document Title" {
					filename = title + ".oath"
				}
				break
			}
		}
	}
	
	vimIndicator := ""
	if m.document.vim.enabled {
		switch m.document.vim.mode {
		case vimNormal:
			vimIndicator = " [NORMAL]"
		case vimInsert:
			vimIndicator = " [INSERT]"
		case vimVisual:
			vimIndicator = " [VISUAL]"
		case vimCommand:
			vimIndicator = " [COMMAND]"
		}
	}
	
	themeName := fmt.Sprintf(" (%s)", theme.Name)
	content.WriteString(headerStyle.Render("Editor - " + filename + modifiedIndicator + themeName + vimIndicator))
	content.WriteString("\n\n")

	for i, block := range m.document.blocks {
		style := blockStyle
		if i == m.document.currentBlock {
			style = currentBlockStyle
		}

		blockTypeIndicator := ""
		switch block.Type {
		case blockMath:
			blockTypeIndicator = "[MATH] "
		case blockCode:
			blockTypeIndicator = "[CODE] "
		case blockQuote:
			blockTypeIndicator = "[QUOTE] "
		case blockList:
			blockTypeIndicator = "[LIST] "
		case blockRawLaTeX:
			blockTypeIndicator = "[RAW] "
		case blockHeading:
			blockTypeIndicator = "[HEAD] "
		default:
			blockTypeIndicator = "[TEXT] "
		}

		blockContent := blockTypeIndicator + block.Content
		if len(block.Content) == 0 {
			blockContent = blockTypeIndicator + fmt.Sprintf("[Empty %s block]", block.Type)
		}

		if i == m.document.currentBlock && m.document.editor.Focused() {
			editorView := m.document.editor.View()

			if m.document.lsp.showCompletions && len(m.document.lsp.completions) > 0 {
				var completionBox strings.Builder
				completionBox.WriteString(editorView)
				completionBox.WriteString("\n")

				for j, comp := range m.document.lsp.completions {
					compStyle := completionStyle
					if j == m.document.lsp.activeCompletion {
						compStyle = selectedCompletionStyle
					}
					completionBox.WriteString(compStyle.Render(comp.Label + " - " + comp.Detail))
					if comp.Example != "" {
						completionBox.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.Muted).Render("  Example: " + comp.Example))
					}
					completionBox.WriteString("\n")
				}
				content.WriteString(style.Render(completionBox.String()))
			} else {
				content.WriteString(style.Render(editorView))
			}
		} else {
			content.WriteString(style.Render(blockContent))
		}
		content.WriteString("\n")
	}

	if len(m.document.lsp.diagnostics) > 0 {
		content.WriteString("\n")
		errorStyle := lipgloss.NewStyle().Foreground(theme.Error)
		warningStyle := lipgloss.NewStyle().Foreground(theme.Warning)

		for _, diag := range m.document.lsp.diagnostics {
			style := errorStyle
			if diag.Severity == "warning" {
				style = warningStyle
			}
			content.WriteString(style.Render(fmt.Sprintf("Line %d: %s", diag.Line, diag.Message)))
			content.WriteString("\n")
		}
	}

	help := "j/k: navigate blocks | enter: edit | n: new | m: math | c: code | l: list | r: raw\n"
	help += "s: save | e: export | T: theme | V: vim | 1/2/3: view modes | +/-: split | t: timer | q: menu"

	content.WriteString("\n")
	content.WriteString(helpStyle.Render(help))

	return content.String()
}

func (m model) renderPreview(width, height int) string {
	var content strings.Builder
	theme := m.getCurrentTheme()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Secondary).
		Width(width).
		Align(lipgloss.Center)

	mathStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Italic(true)

	headingStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent)

	h1Style := headingStyle.Copy().Foreground(theme.Primary).Underline(true)
	h2Style := headingStyle.Copy().Foreground(theme.Secondary)
	h3Style := headingStyle.Copy().Foreground(theme.Muted)

	codeStyle := lipgloss.NewStyle().
		Background(theme.Muted).
		Foreground(theme.Background).
		Padding(0, 1)

	quoteStyle := lipgloss.NewStyle().
		BorderLeft(true).
		BorderForeground(theme.Accent).
		PaddingLeft(1).
		Italic(true)

	content.WriteString(headerStyle.Render("Preview"))
	content.WriteString("\n\n")

	for i, block := range m.document.blocks {
		var rendered RenderedBlock
		if m.document.needsRefresh || block.Rendered == "" {
			rendered = m.document.renderer.renderLaTeX(block.Content)
			// Note: In a full implementation, you'd update the block.Rendered field
		} else {
			rendered = RenderedBlock{
				Unicode: block.Rendered,
				Errors:  []Diagnostic{},
			}
		}

		blockContent := rendered.Unicode
		if len(rendered.Errors) > 0 {
			errorStyle := lipgloss.NewStyle().Foreground(theme.Error)
			var errorMsgs []string
			for _, err := range rendered.Errors {
				errorMsgs = append(errorMsgs, err.Message)
			}
			blockContent += "\n" + errorStyle.Render("Warning: " + strings.Join(errorMsgs, ", "))
		}

		switch block.Type {
		case blockHeading:
			level := strings.Count(strings.TrimSpace(block.Content), "#")
			title := strings.TrimSpace(strings.TrimLeft(block.Content, "# "))
			
			switch level {
			case 1:
				content.WriteString(h1Style.Render(title))
			case 2:
				content.WriteString(h2Style.Render(title))
			case 3:
				content.WriteString(h3Style.Render(title))
			default:
				content.WriteString(headingStyle.Render(title))
			}
		case blockMath:
			content.WriteString(mathStyle.Render(blockContent))
		case blockCode:
			content.WriteString(codeStyle.Render(blockContent))
		case blockQuote:
			content.WriteString(quoteStyle.Render(blockContent))
		case blockList:
			lines := strings.Split(blockContent, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
					content.WriteString("• " + strings.TrimSpace(line[2:]) + "\n")
				} else if line != "" {
					content.WriteString("• " + line + "\n")
				}
			}
		case blockRawLaTeX:
			content.WriteString(mathStyle.Render(blockContent))
		default:
			text := blockContent
			if strings.Contains(text, "**") {
				boldStyle := lipgloss.NewStyle().Bold(true)
				parts := strings.Split(text, "**")
				for i, part := range parts {
					if i%2 == 1 {
						content.WriteString(boldStyle.Render(part))
					} else {
						content.WriteString(part)
					}
				}
			} else {
				content.WriteString(text)
			}
		}

		if i == m.document.currentBlock {
			content.WriteString(" ← ")
		}
		content.WriteString("\n\n")
	}

	if m.document.needsRefresh {
		m.document.needsRefresh = false
	}

	return content.String()
}

func (m model) viewTimer() string {
	var content strings.Builder
	theme := m.getCurrentTheme()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)
		
	helpStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	if m.remaining > 0 || m.duration == 0 {
		if m.input.Focused() {
			content.WriteString(titleStyle.Render("Set Timer Duration") + "\n")
			content.WriteString(m.input.View() + "\n\n")
			content.WriteString(helpStyle.Render("Press enter to start timer."))
		} else {
			timerStr := formatDuration(m.remaining)
			timerStyle := lipgloss.NewStyle().
				Bold(true).
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Primary)

			if m.paused {
				timerStyle = timerStyle.Foreground(theme.Warning)
			}

			content.WriteString(timerStyle.Render(timerStr) + "\n\n")
			help := "p: pause | r: resume | w: edit duration | n: notes | q: back"
			content.WriteString(helpStyle.Render(help))
		}
	} else {
		timerStyle := lipgloss.NewStyle().
			Bold(true).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			Foreground(theme.Error).
			BorderForeground(theme.Error)

		content.WriteString(timerStyle.Render("Time's Up") + "\n\n")
		content.WriteString(helpStyle.Render("Press q to return to editor"))
	}

	content.WriteString("\n\n" + m.notes.View())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content.String(),
	)
}

func (m model) viewExport() string {
	var content strings.Builder
	theme := m.getCurrentTheme()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(m.width).
		Align(lipgloss.Center)

	selectedStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted)

	content.WriteString(titleStyle.Render("Export Document"))
	content.WriteString("\n\nSelect export format:\n\n")

	for i, format := range m.export.formats {
		cursor := "  "
		if i == m.export.selected {
			cursor = "> "
		}

		line := cursor + format
		if i == m.export.selected {
			line = selectedStyle.Render(line)
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	if m.export.input.Focused() {
		content.WriteString("\nFilename: ")
		content.WriteString(m.export.input.View())
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render("Enter filename and press enter to export"))
	} else {
		content.WriteString("\n")
		content.WriteString(helpStyle.Render("j/k: navigate | enter: set filename | q: back"))
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content.String(),
	)
}

func (m model) getCurrentTheme() Theme {
	if theme, exists := themes[m.theme.currentTheme]; exists {
		return theme
	}
	return themes["default"]
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Application crashed: %v\n", r)
			os.Exit(1)
		}
	}()

	model := initialModel()
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
