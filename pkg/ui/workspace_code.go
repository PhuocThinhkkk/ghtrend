package ui

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func isMarkdownPath(p string) bool {
	lower := strings.ToLower(strings.TrimSpace(p))
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".mdx")
}

func (w *workspaceModel) rebuildVisibleNodes() {
	w.visibleNodes = nil
	if len(w.dirChildren) > 0 && len(w.allTree) == 0 {
		w.appendDirChildrenNodes("", 0)
		return
	}

	childrenByParent := make(map[string][]ghclient.TreeEntry)
	for _, entry := range w.allTree {
		parent := path.Dir(entry.Path)
		if parent == "." {
			parent = ""
		}
		childrenByParent[parent] = append(childrenByParent[parent], entry)
	}

	for parent := range childrenByParent {
		entries := childrenByParent[parent]
		sort.Slice(entries, func(i, j int) bool {
			di := entries[i].Type == "tree"
			dj := entries[j].Type == "tree"
			if di != dj {
				return di
			}
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		})
		childrenByParent[parent] = entries
	}

	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		list, ok := w.dirChildren[parent]
		if !ok {
			list = childrenByParent[parent]
		}
		for _, entry := range list {
			isExp := entry.Type == "tree" && w.expanded[entry.Path]
			w.visibleNodes = append(w.visibleNodes, treeNode{
				entry:    entry,
				depth:    depth,
				expanded: isExp,
			})
			if isExp {
				walk(entry.Path, depth+1)
			}
		}
	}
	walk("", 0)
}

func (w *workspaceModel) appendDirChildrenNodes(parent string, depth int) {
	for _, entry := range w.dirChildren[parent] {
		isExp := entry.Type == "tree" && w.expanded[entry.Path]
		w.visibleNodes = append(w.visibleNodes, treeNode{
			entry:    entry,
			depth:    depth,
			expanded: isExp,
		})
		if isExp {
			w.appendDirChildrenNodes(entry.Path, depth+1)
		}
	}
}

func (w *workspaceModel) clampTreeCursor(visibleRows int) {
	if len(w.visibleNodes) == 0 {
		w.treeCursor = 0
		w.treeOffset = 0
		return
	}
	if w.treeCursor < 0 {
		w.treeCursor = 0
	}
	if w.treeCursor >= len(w.visibleNodes) {
		w.treeCursor = len(w.visibleNodes) - 1
	}
	if visibleRows < 1 {
		visibleRows = 1
	}
	if w.treeCursor < w.treeOffset {
		w.treeOffset = w.treeCursor
	}
	if w.treeCursor >= w.treeOffset+visibleRows {
		w.treeOffset = w.treeCursor - visibleRows + 1
	}
	maxOffset := max(0, len(w.visibleNodes)-visibleRows)
	if w.treeOffset > maxOffset {
		w.treeOffset = maxOffset
	}
}

func (m *Model) updateWorkspaceCode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "R":
		m.workspace.viewerPath = ""
		m.workspace.viewerTitle = "README"
		m.workspace.viewerErr = nil
		m.workspace.viewerLoading = false
		m.workspace.viewerIsBinary = false
		m.workspace.viewerTruncated = false
		m.refreshWorkspaceViewerContent()
		return nil

	case "right":
		if m.workspace.codeFocus == wsFocusTree {
			if cmd := m.activateTreeSelection(false); cmd != nil {
				return cmd
			}
			m.workspace.codeFocus = wsFocusViewer
		}
		return nil

	case "left":
		if m.workspace.codeFocus == wsFocusViewer {
			m.workspace.codeFocus = wsFocusTree
			return nil
		}
		return m.collapseOrGoParentTreeNode()
	}

	if m.workspace.codeFocus == wsFocusViewer {
		switch msg.String() {
		case "backspace":
			m.workspace.codeFocus = wsFocusTree
			return nil
		case "g":
			m.workspace.viewer.GotoTop()
			return nil
		case "G":
			m.workspace.viewer.GotoBottom()
			return nil
		default:
			var vpCmd tea.Cmd
			m.workspace.viewer, vpCmd = m.workspace.viewer.Update(msg)
			return vpCmd
		}
	}

	// Tree focused
	switch msg.String() {
	case "up", "k":
		if m.workspace.treeCursor > 0 {
			m.workspace.treeCursor--
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		}
		return nil
	case "down", "j":
		if m.workspace.treeCursor < len(m.workspace.visibleNodes)-1 {
			m.workspace.treeCursor++
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		}
		return nil
	case "enter", "l", " ":
		return m.activateTreeSelection(true)
	case "backspace", "h":
		return m.collapseOrGoParentTreeNode()
	}
	return nil
}

func (m *Model) activateTreeSelection(toggleDir bool) tea.Cmd {
	if len(m.workspace.visibleNodes) == 0 || m.workspace.treeCursor < 0 || m.workspace.treeCursor >= len(m.workspace.visibleNodes) {
		return nil
	}
	node := m.workspace.visibleNodes[m.workspace.treeCursor]
	if node.entry.Type == "tree" {
		if toggleDir && m.workspace.expanded[node.entry.Path] {
			delete(m.workspace.expanded, node.entry.Path)
			m.workspace.rebuildVisibleNodes()
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
			return nil
		}
		m.workspace.expanded[node.entry.Path] = true
		if m.workspace.treeTruncated || (len(m.workspace.dirChildren) > 0 && len(m.workspace.allTree) == 0) {
			if _, ok := m.workspace.dirChildren[node.entry.Path]; !ok {
				svc := m.activeWorkspaceService()
				return wsFetchDirCmd(svc, m.workspace.seq, m.workspace.refSeq, m.workspace.owner, m.workspace.repo, node.entry.Path, m.workspaceActiveRefOrHEAD())
			}
		}
		m.workspace.rebuildVisibleNodes()
		m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		return nil
	}

	// File selected -> fetch and focus viewer
	m.workspace.viewerPath = node.entry.Path
	m.workspace.viewerTitle = node.entry.Name
	m.workspace.viewerLoading = true
	m.workspace.viewerErr = nil
	m.workspace.viewerIsBinary = false
	m.workspace.viewerTruncated = false
	m.workspace.codeFocus = wsFocusViewer
	m.workspace.viewer.SetContent("Loading " + node.entry.Path + "...")

	svc := m.activeWorkspaceService()
	return wsFetchFileCmd(svc, m.workspace.seq, m.workspace.refSeq, m.workspace.owner, m.workspace.repo, node.entry.Path, m.workspaceActiveRefOrHEAD())
}

func (m *Model) collapseOrGoParentTreeNode() tea.Cmd {
	if len(m.workspace.visibleNodes) == 0 || m.workspace.treeCursor < 0 || m.workspace.treeCursor >= len(m.workspace.visibleNodes) {
		return nil
	}
	node := m.workspace.visibleNodes[m.workspace.treeCursor]
	if node.entry.Type == "tree" && m.workspace.expanded[node.entry.Path] {
		delete(m.workspace.expanded, node.entry.Path)
		m.workspace.rebuildVisibleNodes()
		m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		return nil
	}
	parent := path.Dir(node.entry.Path)
	if parent == "." || parent == "" {
		return nil
	}
	for i, n := range m.workspace.visibleNodes {
		if n.entry.Path == parent {
			m.workspace.treeCursor = i
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
			break
		}
	}
	return nil
}

func (m *Model) refreshWorkspaceViewerContent() {
	wrapW := max(20, m.workspace.viewer.Width)
	if m.workspace.viewerPath == "" {
		m.workspace.viewerTitle = "README"
		if !m.workspace.readmeLoaded {
			m.workspace.viewer.SetContent("_Loading repository README..._")
			return
		}
		if strings.TrimSpace(m.workspace.readmeRaw) == "" {
			m.workspace.viewer.SetContent("_No README found for this ref._")
			return
		}
		m.workspace.viewer.SetContent(RenderFullMarkdown(m.workspace.readmeRaw, m.theme, wrapW))
		m.workspace.viewer.GotoTop()
		return
	}

	if m.workspace.viewerLoading {
		m.workspace.viewer.SetContent("Loading " + m.workspace.viewerPath + "...")
		return
	}
	if m.workspace.viewerErr != nil {
		m.workspace.viewer.SetContent("❌ Error loading file:\n\n" + m.workspace.viewerErr.Error())
		return
	}
	if m.workspace.viewerIsBinary {
		m.workspace.viewer.SetContent(fmt.Sprintf("Binary file (%d bytes) — preview skipped.", m.workspace.viewerFileSize))
		return
	}

	var rendered string
	if m.workspace.viewerIsMarkdown {
		rendered = RenderFullMarkdown(m.workspace.viewerRawContent, m.theme, wrapW)
	} else {
		rendered = RenderHighlightedCode(m.workspace.viewerPath, m.workspace.viewerRawContent, m.theme, wrapW)
	}
	if m.workspace.viewerTruncated {
		rendered += "\n\n[File truncated at 512 KiB preview limit]"
	}
	m.workspace.viewer.SetContent(rendered)
	m.workspace.viewer.GotoTop()
}

func (m Model) renderWorkspaceCodeTab(width, height int) string {
	if width >= 95 {
		treeW := max(26, min(42, width*30/100))
		viewerW := width - treeW
		treeBox := m.renderWorkspaceTreeBox(treeW, height)
		viewerBox := m.renderWorkspaceViewerBox(viewerW, height)
		return lipgloss.JoinHorizontal(lipgloss.Top, treeBox, viewerBox)
	}
	treeH := max(5, height*40/100)
	viewerH := max(4, height-treeH)
	treeBox := m.renderWorkspaceTreeBox(width, treeH)
	viewerBox := m.renderWorkspaceViewerBox(width, viewerH)
	return lipgloss.JoinVertical(lipgloss.Left, treeBox, viewerBox)
}

func (m Model) renderWorkspaceTreeBox(width, height int) string {
	focused := m.workspace.codeFocus == wsFocusTree
	innerW := max(10, width-4)
	visibleRows := max(1, height-2)

	mutedStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	title := "Files"
	if m.workspace.treeTruncated {
		title = "Files (partial)"
	}

	if !m.workspace.treeLoaded {
		return WrapInTitledBorder("\n "+mutedStyle.Render("Loading directory tree..."), width, height, title, focused, m.theme)
	}
	if m.workspace.treeErr != nil {
		return WrapInTitledBorder("\n "+lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(TruncateDisplayWidth("⚠ "+m.workspace.treeErr.Error(), innerW, "…")), width, height, title, focused, m.theme)
	}
	if len(m.workspace.visibleNodes) == 0 {
		return WrapInTitledBorder("\n "+mutedStyle.Render("Repository is empty."), width, height, title, focused, m.theme)
	}

	wCopy := m.workspace
	wCopy.clampTreeCursor(visibleRows)
	end := min(len(wCopy.visibleNodes), wCopy.treeOffset+visibleRows)

	var lines []string
	for i := wCopy.treeOffset; i < end; i++ {
		node := wCopy.visibleNodes[i]
		indent := strings.Repeat("  ", min(node.depth, 6))
		icon := "📄"
		if node.entry.Type == "tree" {
			if node.expanded {
				icon = "📂"
			} else {
				icon = "📁"
			}
		}
		raw := fmt.Sprintf("%s%s %s", indent, icon, node.entry.Name)
		clipped := TruncateDisplayWidth(raw, innerW, "…")
		if i == wCopy.treeCursor {
			if focused {
				lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Render(" "+padRightDisplay(clipped, innerW)))
			} else {
				lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Primary).Render(" "+clipped))
			}
		} else {
			color := m.theme.Text
			if node.entry.Type == "tree" {
				color = m.theme.Secondary
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(color).Render(" "+clipped))
		}
	}
	return WrapInTitledBorder(strings.Join(lines, "\n"), width, height, title, focused, m.theme)
}

func (m Model) renderWorkspaceViewerBox(width, height int) string {
	focused := m.workspace.codeFocus == wsFocusViewer
	title := m.workspace.viewerTitle
	if m.workspace.viewerPath != "" {
		title = m.workspace.viewerPath
	}
	if title == "" {
		title = "README"
	}
	content := m.workspace.viewer.View()
	return WrapInTitledBorder(content, width, height, title, focused, m.theme)
}
