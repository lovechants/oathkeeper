## Oathkeeper 
---

This is just a simple tui tool for having a timer + simple todo notes box

Just to help with staying focused for periods of time in terminal when not using tmux to track time 

May update it with features as needed. 

Written in go with bubbletea, opposite of [Aliyah](https://github.com/lovechants/Aliyah/tree/main) which uses rust and ratatui

---

## How to use 
```bash
git clone 
go run . #if you want to run it in the directory only 
```

```bash
git clone 
go install # if you want to run system wide 
```
> If you haven't added the Go bin directory add it to your shells config file
```zsh
export PATH=$PATH:$(go env GOPATH)/bin
source ~/.zshrc #if using zsh but you get the idea
```

---
Update 1 Tex editor. 

## Features

- **Block-based editing**: Organize content into structured blocks (headings, text, math, code, lists)
- **Mathematical notation**: Write LaTeX-style math with `$inline$` and `$$display$$` syntax 
    - This is still a work in progress {} sometimes renders as ```{ *```
- **Live preview**: Split-pane view with rendered preview alongside editor
- **Multiple export formats**: PDF, HTML, Unicode text, and Markdown
- **File browser**: Built-in file navigation and management
- **Themes**: Multiple color schemes including default, gruvbox, nord, and dracula
- **Document templates**: Quick start templates for different document types

## Installation

### Prerequisites

- Go 1.19 or later
- LaTeX distribution (for PDF export)
  - macOS: Install TinyTeX or MacTeX
  - Linux: Install texlive
  - Windows: Install MiKTeX or TeX Live

### Install from source

```bash
git clone https://github.com/yourusername/oathkeeper.git
cd oathkeeper
go install .
```

Make sure `$HOME/go/bin` is in your PATH:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

## Usage

Start the application:

```bash
oathkeeper
```

### Navigation

- `j/k` or arrow keys: Navigate between items
- `enter`: Select item or edit block
- `esc`: Exit edit mode or go back
- `q`: Quit or go back to previous view

### Editing

- `n`: Create new block
- `m`: Convert block to math
- `c`: Convert block to code
- `l`: Convert block to list
- `r`: Convert block to raw LaTeX
- `s`: Save document
- `d`: Delete current block

### View modes

- `1`: Editor only
- `2`: Split pane (default)
- `3`: Preview only
- `+/-`: Adjust split ratio

### Export

- `e`: Export document
- Choose format: PDF, HTML, Unicode text, or Markdown
- Enter filename (or leave blank for auto-generated name)

### Mathematical notation

Use standard LaTeX syntax within math blocks:

```
Inline math: $f(x) = x^2 + 1$

Display equations:
$$\int_0^1 f(x) dx = \frac{1}{2}$$

Mixed content:
Given functions $f$ and $g$, their sum is $f + g$.
```

### Document structure

Documents are saved as `.oath` files containing JSON with your content blocks and metadata. The format preserves block types, mathematical content, and document structure.

### Themes

- `T`: Cycle through available themes
- Themes persist between sessions
- Available: default, gruvbox, nord, dracula

## File formats

- **Native**: `.oath` files (JSON-based)
- **Export**: PDF, HTML, Markdown, Unicode text
- **Import**: Currently supports `.oath` files only

## Configuration

Settings are automatically saved to `~/.oathkeeper/preferences.json` including:

- Last used directory
- Theme preference
- View mode settings
- Split pane ratio

## Troubleshooting

### PDF export not working

Ensure LaTeX is properly installed:

```bash
# Test LaTeX installation
pdflatex --version

# Install missing packages (TinyTeX)
tlmgr install listings xcolor
```

### Math not rendering

Check that you're using proper delimiters:
- Single `$` for inline math
- Double `$$` for display equations
- Ensure balanced delimiters

### Performance issues

- Large documents may render slowly in preview mode
- Use editor-only view (`1`) for better performance while editing
- Split view works best for documents under 100 blocks

## License

MIT License - see LICENSE file for details.

Thanks
