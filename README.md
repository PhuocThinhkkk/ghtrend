<a id="readme-top"></a>

<div align="center">
  <h1 align="center">ghtrend</h1>

  <p align="center">
  Explore GitHub trending and user repositories directly from your terminal with style.
  </p>
</div>

## Demo
![Demo](assets/demo.gif)

## Features
- 🚀 **GitHub Trending**: Explore trending repositories by daily, weekly, or monthly frequency and filter by language.
- 👤 **User Repository Explorer**: Browse public repositories of any GitHub user (`--user` / `-u`).
- 🎨 **Aesthetic Theme Engine**: 7 curated themes (`catppuccin`, `tokyonight`, `dracula`, `nord`, `gruvbox`, `cyberpunk`, `classic`) with interactive in-app modal (`[t]`) and config persistence.
- ⏳ **Animated Loading**: Smooth gradient progress bar and spinner during network and cache hydration.
- 📱 **Adaptive Responsive UI**: Dual-pane layout on wide screens, adaptive tabbed view on compact displays, and dedicated scrollable README viewport.
- ⚡ **Local Caching**: Instant startup with 60-minute cache and `--no-cache` override.

## Requirements
- Go **1.24+**
- Git

## Installation

### Option 1 — Install with Go (Linux, macOS, Windows)
Make sure Go is installed, then run:

```bash
go install github.com/PhuocThinhkkk/ghtrend@latest
```
This will build and install `ghtrend` into your `$GOBIN` (default: `$HOME/go/bin` or `%USERPROFILE%\go\bin`).

### Option 2 — Install via script (Linux/macOS only)
#### Quick install (curl):
```bash
curl -fsSL https://raw.githubusercontent.com/PhuocThinhkkk/ghtrend/main/install.sh | bash
```

#### Manual install:
```bash
git clone https://github.com/PhuocThinhkkk/ghtrend.git
cd ghtrend
chmod +x install.sh
./install.sh
```

## Usage

**Explore GitHub Trending:**
```bash
ghtrend
```

**Explore a specific user's repositories:**
```bash
ghtrend -u PhuocThinhkkk
ghtrend --user torvalds --limit 10
```

**Filter by programming language:**
```bash
ghtrend --lang go --limit 5
ghtrend -u octocat --lang rust
```

**Show trending for the past week or month:**
```bash
ghtrend --since weekly
ghtrend --since monthly
```

**Start with a specific theme:**
```bash
ghtrend --theme tokyonight
ghtrend -t dracula
```
*Available themes: `catppuccin`, `tokyonight`, `dracula`, `nord`, `gruvbox`, `cyberpunk`, `classic`.*

**Bypass cache and fetch fresh data:**
```bash
ghtrend --no-cache
```

**View all options:**
```bash
ghtrend --help
```

## Keyboard Shortcuts

| Shortcut | Action |
|:---|:---|
| `↑` / `↓` or `k` / `j` | Navigate repository or file list |
| `Tab` / `Shift+Tab` | Cycle focus between Repos, Files, and Readme panes |
| `1` | Jump focus to Repositories table |
| `2` | Jump focus to Files list |
| `3` | Jump focus to Readme viewport |
| `PgUp` / `PgDn` | Scroll Readme preview |
| `t` | Open interactive Theme Selector modal |
| `Enter` | Select theme in modal |
| `Esc` | Close modal / return |
| `r` | Retry loading on failure |
| `q` / `Ctrl+C` | Quit |

## Configuration
Themes and preferences are persisted to:
- **Linux/macOS**: `~/.config/ghtrend/config.json`
- **Windows**: `%APPDATA%\ghtrend\config.json`
