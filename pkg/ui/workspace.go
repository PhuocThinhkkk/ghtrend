package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type workspaceTab int8

const (
	wsTabCode workspaceTab = iota
	wsTabPRs
	wsTabIssues
	wsTabCommits
)

type workspaceCodeFocus int8

const (
	wsFocusTree workspaceCodeFocus = iota
	wsFocusViewer
)

// WorkspaceService provides GitHub API operations for the Repository Workspace.
type WorkspaceService struct {
	FetchMetadata      func(ctx context.Context, owner, repo string) (ghclient.RepoMetadata, error)
	FetchRefs          func(ctx context.Context, owner, repo string) ([]ghclient.RefInfo, error)
	FetchTree          func(ctx context.Context, owner, repo, ref string) ([]ghclient.TreeEntry, bool, error)
	FetchDirContents   func(ctx context.Context, owner, repo, dirPath, ref string) ([]ghclient.TreeEntry, error)
	FetchFile          func(ctx context.Context, owner, repo, filePath, ref string) (ghclient.FileContent, error)
	FetchREADME        func(ctx context.Context, owner, repo, ref string) (string, error)
	FetchPRs           func(ctx context.Context, owner, repo, state string, page int) ([]ghclient.PullRequest, bool, error)
	FetchPRTimeline    func(ctx context.Context, owner, repo string, number int) ([]ghclient.TimelineComment, error)
	FetchIssues        func(ctx context.Context, owner, repo, state string, page int) ([]ghclient.Issue, bool, error)
	FetchIssueTimeline func(ctx context.Context, owner, repo string, number int) ([]ghclient.TimelineComment, error)
	CreateComment      func(ctx context.Context, owner, repo string, number int, body string) (ghclient.TimelineComment, error)
	UpdateIssueState   func(ctx context.Context, owner, repo string, number int, state string) (ghclient.Issue, error)
	FetchCommits       func(ctx context.Context, owner, repo, ref string, page int) ([]ghclient.CommitSummary, bool, error)
	FetchCommitDiff    func(ctx context.Context, owner, repo, sha string) (ghclient.CommitDiff, error)
	IsStarred          func(ctx context.Context, owner, repo string) (bool, error)
	SetStarred         func(ctx context.Context, owner, repo string, star bool) error
	CloneRepo          func(ctx context.Context, owner, repo, parentDir string) (string, error)
}

func defaultWorkspaceService() WorkspaceService {
	return WorkspaceService{
		FetchMetadata:      ghclient.FetchRepoMetadata,
		FetchRefs:          ghclient.FetchRepoRefs,
		FetchTree:          ghclient.FetchRepoTree,
		FetchDirContents:   ghclient.FetchRepoDirContents,
		FetchFile:          ghclient.FetchFileContent,
		FetchREADME:        ghclient.FetchRepoREADME,
		FetchPRs:           ghclient.FetchPullRequests,
		FetchPRTimeline:    ghclient.FetchPullRequestTimeline,
		FetchIssues:        ghclient.FetchIssues,
		FetchIssueTimeline: ghclient.FetchIssueTimeline,
		CreateComment:      ghclient.CreateIssueComment,
		UpdateIssueState:   ghclient.UpdateIssueState,
		FetchCommits:       ghclient.FetchRepoCommits,
		FetchCommitDiff:    ghclient.FetchCommitDiff,
		IsStarred:          ghclient.IsRepoStarred,
		SetStarred:         ghclient.SetRepoStarred,
		CloneRepo:          utils.CloneRepository,
	}
}

func (m Model) activeWorkspaceService() WorkspaceService {
	if m.launchOpts.Workspace != nil {
		svc := *m.launchOpts.Workspace
		def := defaultWorkspaceService()
		if svc.FetchMetadata == nil {
			svc.FetchMetadata = def.FetchMetadata
		}
		if svc.FetchRefs == nil {
			svc.FetchRefs = def.FetchRefs
		}
		if svc.FetchTree == nil {
			svc.FetchTree = def.FetchTree
		}
		if svc.FetchDirContents == nil {
			svc.FetchDirContents = def.FetchDirContents
		}
		if svc.FetchFile == nil {
			svc.FetchFile = def.FetchFile
		}
		if svc.FetchREADME == nil {
			svc.FetchREADME = def.FetchREADME
		}
		if svc.FetchPRs == nil {
			svc.FetchPRs = def.FetchPRs
		}
		if svc.FetchPRTimeline == nil {
			svc.FetchPRTimeline = def.FetchPRTimeline
		}
		if svc.FetchIssues == nil {
			svc.FetchIssues = def.FetchIssues
		}
		if svc.FetchIssueTimeline == nil {
			svc.FetchIssueTimeline = def.FetchIssueTimeline
		}
		if svc.CreateComment == nil {
			svc.CreateComment = def.CreateComment
		}
		if svc.UpdateIssueState == nil {
			svc.UpdateIssueState = def.UpdateIssueState
		}
		if svc.FetchCommits == nil {
			svc.FetchCommits = def.FetchCommits
		}
		if svc.FetchCommitDiff == nil {
			svc.FetchCommitDiff = def.FetchCommitDiff
		}
		if svc.IsStarred == nil {
			svc.IsStarred = def.IsStarred
		}
		if svc.SetStarred == nil {
			svc.SetStarred = def.SetStarred
		}
		if svc.CloneRepo == nil {
			svc.CloneRepo = def.CloneRepo
		}
		return svc
	}
	return defaultWorkspaceService()
}

type treeNode struct {
	entry    ghclient.TreeEntry
	depth    int
	expanded bool
}

type workspaceModel struct {
	seq        uint64
	refSeq     uint64
	owner      string
	repo       string
	activeRef  string
	breadcrumb string
	prevState  appState
	tab        workspaceTab
	codeFocus  workspaceCodeFocus

	meta       ghclient.RepoMetadata
	metaLoaded bool
	metaErr    error

	refs            []ghclient.RefInfo
	refsLoaded      bool
	showBranchModal bool
	branchFilter    textinput.Model
	filteredRefs    []ghclient.RefInfo
	branchIndex     int

	allTree       []ghclient.TreeEntry
	treeTruncated bool
	treeLoaded    bool
	treeErr       error
	expanded      map[string]bool
	dirChildren   map[string][]ghclient.TreeEntry
	visibleNodes  []treeNode
	treeCursor    int
	treeOffset    int

	viewer           viewport.Model
	viewerTitle      string
	viewerPath       string
	viewerRawContent string
	viewerIsMarkdown bool
	viewerIsBinary   bool
	viewerTruncated  bool
	viewerFileSize   int64
	viewerLoading    bool
	viewerErr        error

	readmeRaw    string
	readmeLoaded bool

	prSeq             uint64
	prs               []ghclient.PullRequest
	prsLoaded         bool
	prsLoading        bool
	prsErr            error
	prFilter          string
	prPage            int
	prHasNext         bool
	prCursor          int
	prOffset          int
	prDetail          *ghclient.PullRequest
	prTimeline        []ghclient.TimelineComment
	prTimelineLoading bool
	prTimelineErr     error
	prViewport        viewport.Model

	issueSeq             uint64
	issues               []ghclient.Issue
	issuesLoaded         bool
	issuesLoading        bool
	issuesErr            error
	issueFilter          string
	issuePage            int
	issueHasNext         bool
	issueCursor          int
	issueOffset          int
	issueDetail          *ghclient.Issue
	issueTimeline        []ghclient.TimelineComment
	issueTimelineLoading bool
	issueTimelineErr     error
	issueViewport        viewport.Model
	issueStateUpdating   bool

	commentSeq          uint64
	showCommentModal    bool
	commentTargetKind   string
	commentTargetNumber int
	commentInput        textarea.Model
	commentSubmitting   bool
	commentErr          error

	commitSeq          uint64
	commits            []ghclient.CommitSummary
	commitsLoaded      bool
	commitsLoading     bool
	commitsErr         error
	commitPage         int
	commitHasNext      bool
	commitCursor       int
	commitOffset       int
	commitDiffSeq      uint64
	commitDiffOpen     bool
	commitDiffLoading  bool
	commitDiffErr      error
	commitDiff         ghclient.CommitDiff
	commitDiffFocus    workspaceCodeFocus
	commitFileCursor   int
	commitFileOffset   int
	commitDiffViewport viewport.Model

	starSeq      uint64
	starLoaded   bool
	starred      bool
	starUpdating bool
	cloning      bool
}

type wsMetadataLoadedMsg struct {
	seq   uint64
	owner string
	repo  string
	meta  ghclient.RepoMetadata
	err   error
}

type wsRefsLoadedMsg struct {
	seq   uint64
	owner string
	repo  string
	refs  []ghclient.RefInfo
	err   error
}

type wsTreeLoadedMsg struct {
	seq       uint64
	refSeq    uint64
	owner     string
	repo      string
	ref       string
	entries   []ghclient.TreeEntry
	truncated bool
	err       error
}

type wsDirLoadedMsg struct {
	seq     uint64
	refSeq  uint64
	owner   string
	repo    string
	dirPath string
	ref     string
	entries []ghclient.TreeEntry
	err     error
}

type wsREADMELoadedMsg struct {
	seq     uint64
	refSeq  uint64
	owner   string
	repo    string
	ref     string
	content string
	err     error
}

type wsFileLoadedMsg struct {
	seq      uint64
	refSeq   uint64
	owner    string
	repo     string
	filePath string
	ref      string
	file     ghclient.FileContent
	err      error
}

func (m *Model) openWorkspace(owner, repo, breadcrumb string, prev appState) tea.Cmd {
	m.launchSeq++
	m.cursorDebouncer.Cancel()
	seq := m.workspace.seq + 1
	refSeq := m.workspace.refSeq + 1
	starSeq := m.workspace.starSeq + 1

	bf := textinput.New()
	bf.Placeholder = "filter branches or tags..."
	bf.Prompt = "🌿 "
	bf.CharLimit = 120
	bf.Width = 36

	vp := viewport.New(60, 16)
	vp.SetContent("_Loading repository README..._")
	prVP := viewport.New(60, 16)
	issueVP := viewport.New(60, 16)
	commitVP := viewport.New(60, 16)

	m.workspace = workspaceModel{
		seq:                seq,
		refSeq:             refSeq,
		owner:              owner,
		repo:               repo,
		activeRef:          "",
		breadcrumb:         breadcrumb,
		prevState:          prev,
		tab:                wsTabCode,
		codeFocus:          wsFocusTree,
		branchFilter:       bf,
		expanded:           make(map[string]bool),
		dirChildren:        make(map[string][]ghclient.TreeEntry),
		viewer:             vp,
		viewerTitle:        "README",
		prFilter:           "open",
		prPage:             1,
		prViewport:         prVP,
		issueFilter:        "open",
		issuePage:          1,
		issueViewport:      issueVP,
		commitPage:         1,
		commitDiffViewport: commitVP,
		starSeq:            starSeq,
	}

	m.state = stateWorkspace
	m.toastMessage = ""
	m.toastID++
	m.recalculateWorkspaceLayout(m.width, m.height)

	svc := m.activeWorkspaceService()
	return tea.Batch(
		wsFetchMetadataCmd(svc, seq, owner, repo),
		wsFetchRefsCmd(svc, seq, owner, repo),
		wsFetchTreeCmd(svc, seq, refSeq, owner, repo, "HEAD"),
		wsFetchREADMECmd(svc, seq, refSeq, owner, repo, ""),
		wsFetchStarStatusCmd(svc, seq, starSeq, owner, repo),
	)
}

func wsFetchMetadataCmd(svc WorkspaceService, seq uint64, owner, repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		meta, err := svc.FetchMetadata(ctx, owner, repo)
		return wsMetadataLoadedMsg{seq: seq, owner: owner, repo: repo, meta: meta, err: err}
	}
}

func wsFetchRefsCmd(svc WorkspaceService, seq uint64, owner, repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		refs, err := svc.FetchRefs(ctx, owner, repo)
		return wsRefsLoadedMsg{seq: seq, owner: owner, repo: repo, refs: refs, err: err}
	}
}

func wsFetchTreeCmd(svc WorkspaceService, seq, refSeq uint64, owner, repo, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		entries, truncated, err := svc.FetchTree(ctx, owner, repo, ref)
		return wsTreeLoadedMsg{seq: seq, refSeq: refSeq, owner: owner, repo: repo, ref: ref, entries: entries, truncated: truncated, err: err}
	}
}

func wsFetchDirCmd(svc WorkspaceService, seq, refSeq uint64, owner, repo, dirPath, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		entries, err := svc.FetchDirContents(ctx, owner, repo, dirPath, ref)
		return wsDirLoadedMsg{seq: seq, refSeq: refSeq, owner: owner, repo: repo, dirPath: dirPath, ref: ref, entries: entries, err: err}
	}
}

func wsFetchREADMECmd(svc WorkspaceService, seq, refSeq uint64, owner, repo, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		content, err := svc.FetchREADME(ctx, owner, repo, ref)
		return wsREADMELoadedMsg{seq: seq, refSeq: refSeq, owner: owner, repo: repo, ref: ref, content: content, err: err}
	}
}

func wsFetchFileCmd(svc WorkspaceService, seq, refSeq uint64, owner, repo, filePath, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		file, err := svc.FetchFile(ctx, owner, repo, filePath, ref)
		return wsFileLoadedMsg{seq: seq, refSeq: refSeq, owner: owner, repo: repo, filePath: filePath, ref: ref, file: file, err: err}
	}
}

func (m *Model) switchWorkspaceRef(newRef string) tea.Cmd {
	newRef = strings.TrimSpace(newRef)
	if newRef == "" {
		return nil
	}
	m.workspace.activeRef = newRef
	m.workspace.showBranchModal = false
	m.workspace.branchFilter.Reset()
	m.workspace.branchFilter.Blur()

	m.workspace.allTree = nil
	m.workspace.visibleNodes = nil
	m.workspace.treeLoaded = false
	m.workspace.treeErr = nil
	m.workspace.treeCursor = 0
	m.workspace.treeOffset = 0
	m.workspace.expanded = make(map[string]bool)
	m.workspace.dirChildren = make(map[string][]ghclient.TreeEntry)

	m.workspace.viewerPath = ""
	m.workspace.viewerTitle = "README"
	m.workspace.viewerRawContent = ""
	m.workspace.viewerIsMarkdown = true
	m.workspace.viewerIsBinary = false
	m.workspace.viewerTruncated = false
	m.workspace.viewerErr = nil
	m.workspace.viewerLoading = false
	m.workspace.readmeLoaded = false
	m.workspace.readmeRaw = ""
	m.workspace.viewer.SetContent("_Loading README for " + newRef + "..._")

	m.workspace.refSeq++
	m.workspace.commits = nil
	m.workspace.commitsLoaded = false
	m.workspace.commitsLoading = false
	m.workspace.commitsErr = nil
	m.workspace.commitPage = 1
	m.workspace.commitCursor = 0
	m.workspace.commitOffset = 0
	m.workspace.commitDiffSeq++
	m.workspace.commitDiffOpen = false

	svc := m.activeWorkspaceService()
	seq := m.workspace.seq
	refSeq := m.workspace.refSeq
	cmds := []tea.Cmd{
		wsFetchTreeCmd(svc, seq, refSeq, m.workspace.owner, m.workspace.repo, newRef),
		wsFetchREADMECmd(svc, seq, refSeq, m.workspace.owner, m.workspace.repo, newRef),
		m.setTransientToast("🌿 Switched to "+newRef, 2*time.Second),
	}
	if m.workspace.tab == wsTabCommits {
		cmds = append(cmds, m.ensureWorkspaceCommitsLoaded())
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleWorkspaceMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case wsMetadataLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		m.workspace.metaLoaded = true
		m.workspace.metaErr = msg.err
		if msg.err == nil {
			m.workspace.meta = msg.meta
			if m.workspace.activeRef == "" && msg.meta.DefaultBranch != "" {
				m.workspace.activeRef = msg.meta.DefaultBranch
			}
		}
		m.recalculateWorkspaceLayout(m.width, m.height)
		return nil, true

	case wsRefsLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		m.workspace.refsLoaded = true
		if msg.err == nil {
			m.workspace.refs = msg.refs
			m.workspace.applyBranchFilter()
		}
		return nil, true

	case wsTreeLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.refSeq != m.workspace.refSeq {
			return nil, true
		}
		m.workspace.treeLoaded = true
		m.workspace.treeErr = msg.err
		m.workspace.treeTruncated = msg.truncated
		if msg.err == nil {
			m.workspace.allTree = msg.entries
			if msg.truncated && len(msg.entries) == 0 {
				svc := m.activeWorkspaceService()
				return wsFetchDirCmd(svc, m.workspace.seq, m.workspace.refSeq, m.workspace.owner, m.workspace.repo, "", m.workspaceActiveRefOrHEAD()), true
			}
			m.workspace.rebuildVisibleNodes()
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		}
		return nil, true

	case wsDirLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.refSeq != m.workspace.refSeq {
			return nil, true
		}
		if msg.err == nil {
			m.workspace.dirChildren[msg.dirPath] = msg.entries
			m.workspace.rebuildVisibleNodes()
			m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
		} else {
			return m.setTransientToast("⚠ Failed to load directory: "+msg.err.Error(), 3*time.Second), true
		}
		return nil, true

	case wsREADMELoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.refSeq != m.workspace.refSeq {
			return nil, true
		}
		m.workspace.readmeLoaded = true
		if msg.err == nil {
			m.workspace.readmeRaw = msg.content
		}
		if m.workspace.viewerPath == "" {
			m.refreshWorkspaceViewerContent()
		}
		return nil, true

	case wsFileLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.refSeq != m.workspace.refSeq {
			return nil, true
		}
		if msg.filePath != m.workspace.viewerPath {
			return nil, true
		}
		m.workspace.viewerLoading = false
		m.workspace.viewerErr = msg.err
		if msg.err == nil {
			m.workspace.viewerRawContent = msg.file.Content
			m.workspace.viewerIsBinary = msg.file.IsBinary
			m.workspace.viewerTruncated = msg.file.Truncated
			m.workspace.viewerFileSize = msg.file.Size
			m.workspace.viewerIsMarkdown = isMarkdownPath(msg.filePath)
		}
		m.refreshWorkspaceViewerContent()
		return nil, true
	case wsPRsLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.reqSeq != m.workspace.prSeq {
			return nil, true
		}
		m.workspace.prsLoading = false
		m.workspace.prsLoaded = true
		m.workspace.prsErr = msg.err
		if msg.err == nil {
			m.workspace.prs = msg.prs
			m.workspace.prFilter = msg.filter
			m.workspace.prPage = msg.page
			m.workspace.prHasNext = msg.hasNext
			m.workspace.clampPRCursor(m.workspaceListVisibleRows())
		}
		return nil, true
	case wsPRTimelineLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		if m.workspace.prDetail == nil || m.workspace.prDetail.Number != msg.number {
			return nil, true
		}
		m.workspace.prTimelineLoading = false
		m.workspace.prTimelineErr = msg.err
		if msg.err == nil {
			m.workspace.prTimeline = msg.comments
		}
		m.refreshPRDetailContent()
		return nil, true
	case wsIssuesLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.reqSeq != m.workspace.issueSeq {
			return nil, true
		}
		m.workspace.issuesLoading = false
		m.workspace.issuesLoaded = true
		m.workspace.issuesErr = msg.err
		if msg.err == nil {
			m.workspace.issues = msg.issues
			m.workspace.issueFilter = msg.filter
			m.workspace.issuePage = msg.page
			m.workspace.issueHasNext = msg.hasNext
			m.workspace.clampIssueCursor(m.workspaceListVisibleRows())
		}
		return nil, true
	case wsIssueTimelineLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		if m.workspace.issueDetail == nil || m.workspace.issueDetail.Number != msg.number {
			return nil, true
		}
		m.workspace.issueTimelineLoading = false
		m.workspace.issueTimelineErr = msg.err
		if msg.err == nil {
			m.workspace.issueTimeline = msg.comments
		}
		m.refreshIssueDetailContent()
		return nil, true
	case wsCommentPostedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.reqSeq != m.workspace.commentSeq {
			return nil, true
		}
		m.workspace.commentSubmitting = false
		if msg.err != nil {
			m.workspace.commentErr = msg.err
			return nil, true
		}
		m.workspace.showCommentModal = false
		m.workspace.commentInput.Blur()
		if msg.kind == "pr" {
			for i := range m.workspace.prs {
				if m.workspace.prs[i].Number == msg.number {
					m.workspace.prs[i].CommentCount++
				}
			}
			if m.workspace.prDetail != nil && m.workspace.prDetail.Number == msg.number {
				m.workspace.prDetail.CommentCount++
				m.workspace.prTimeline = append(m.workspace.prTimeline, msg.comment)
				m.refreshPRDetailContent()
				return tea.Batch(
					wsFetchPRTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, msg.number),
					m.setTransientToast("💬 Comment posted to PR", 2*time.Second),
				), true
			}
		} else if msg.kind == "issue" {
			for i := range m.workspace.issues {
				if m.workspace.issues[i].Number == msg.number {
					m.workspace.issues[i].CommentCount++
				}
			}
			if m.workspace.issueDetail != nil && m.workspace.issueDetail.Number == msg.number {
				m.workspace.issueDetail.CommentCount++
				m.workspace.issueTimeline = append(m.workspace.issueTimeline, msg.comment)
				m.refreshIssueDetailContent()
				return tea.Batch(
					wsFetchIssueTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, msg.number),
					m.setTransientToast("💬 Comment posted to Issue", 2*time.Second),
				), true
			}
		}
		return m.setTransientToast("💬 Comment posted", 2*time.Second), true
	case wsIssueStateUpdatedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		m.workspace.issueStateUpdating = false
		if msg.err != nil {
			return m.setTransientToast("⚠ "+msg.err.Error(), 3*time.Second), true
		}
		for i := range m.workspace.issues {
			if m.workspace.issues[i].Number == msg.number {
				m.workspace.issues[i].State = msg.issue.State
				m.workspace.issues[i].StateReason = msg.issue.StateReason
			}
		}
		if m.workspace.issueDetail != nil && m.workspace.issueDetail.Number == msg.number {
			m.workspace.issueDetail.State = msg.issue.State
			m.workspace.issueDetail.StateReason = msg.issue.StateReason
			m.refreshIssueDetailContent()
		}
		return m.setTransientToast(fmt.Sprintf("Issue #%d marked %s", msg.number, msg.issue.State), 2*time.Second), true
	case wsCommitsLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.refSeq != m.workspace.refSeq || msg.reqSeq != m.workspace.commitSeq {
			return nil, true
		}
		m.workspace.commitsLoading = false
		m.workspace.commitsLoaded = true
		m.workspace.commitsErr = msg.err
		if msg.err == nil {
			m.workspace.commits = msg.commits
			m.workspace.commitPage = msg.page
			m.workspace.commitHasNext = msg.hasNext
			m.workspace.clampCommitCursor(m.workspaceListVisibleRows())
		}
		return nil, true
	case wsCommitDiffLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.reqSeq != m.workspace.commitDiffSeq {
			return nil, true
		}
		m.workspace.commitDiffLoading = false
		m.workspace.commitDiffErr = msg.err
		if msg.err == nil {
			m.workspace.commitDiff = msg.diff
			m.workspace.commitFileCursor = 0
			m.workspace.commitFileOffset = 0
		}
		m.refreshCommitDiffViewport()
		return nil, true
	case wsStarStatusLoadedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.starSeq != m.workspace.starSeq {
			return nil, true
		}
		if msg.err == nil {
			m.workspace.starLoaded = true
			m.workspace.starred = msg.starred
		}
		return nil, true
	case wsStarToggledMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq || msg.starSeq != m.workspace.starSeq {
			return nil, true
		}
		m.workspace.starUpdating = false
		if msg.err != nil {
			return m.setTransientToast("⚠ Star action failed: "+msg.err.Error(), 3*time.Second), true
		}
		m.workspace.starLoaded = true
		m.workspace.starred = msg.starred
		if msg.starred {
			return m.setTransientToast("★ Starred "+m.workspace.owner+"/"+m.workspace.repo, 2*time.Second), true
		}
		return m.setTransientToast("☆ Unstarred "+m.workspace.owner+"/"+m.workspace.repo, 2*time.Second), true
	case wsCloneCompletedMsg:
		if m.state != stateWorkspace || msg.seq != m.workspace.seq {
			return nil, true
		}
		m.workspace.cloning = false
		if msg.err != nil {
			return m.setTransientToast("⚠ Clone failed: "+msg.err.Error(), 3*time.Second), true
		}
		return m.setTransientToast("✅ Cloned to "+msg.targetPath, 3*time.Second), true
	}
	return nil, false
}

func (m *Model) updateWorkspaceBranchModal(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "b":
		m.workspace.showBranchModal = false
		m.workspace.branchFilter.Reset()
		m.workspace.branchFilter.Blur()
		return nil
	case "up", "ctrl+p":
		if m.workspace.branchIndex > 0 {
			m.workspace.branchIndex--
		}
		return nil
	case "down", "ctrl+n":
		if m.workspace.branchIndex < len(m.workspace.filteredRefs)-1 {
			m.workspace.branchIndex++
		}
		return nil
	case "enter":
		if len(m.workspace.filteredRefs) > 0 && m.workspace.branchIndex >= 0 && m.workspace.branchIndex < len(m.workspace.filteredRefs) {
			chosen := m.workspace.filteredRefs[m.workspace.branchIndex]
			return m.switchWorkspaceRef(chosen.Name)
		}
		m.workspace.showBranchModal = false
		return nil
	default:
		var cmd tea.Cmd
		m.workspace.branchFilter, cmd = m.workspace.branchFilter.Update(msg)
		m.workspace.applyBranchFilter()
		return cmd
	}
}

func (w *workspaceModel) applyBranchFilter() {
	query := strings.ToLower(strings.TrimSpace(w.branchFilter.Value()))
	if query == "" {
		w.filteredRefs = append([]ghclient.RefInfo(nil), w.refs...)
	} else {
		filtered := make([]ghclient.RefInfo, 0, len(w.refs))
		for _, r := range w.refs {
			if strings.Contains(strings.ToLower(r.Name), query) || strings.Contains(strings.ToLower(r.Kind), query) {
				filtered = append(filtered, r)
			}
		}
		w.filteredRefs = filtered
	}
	if w.branchIndex >= len(w.filteredRefs) {
		w.branchIndex = max(0, len(w.filteredRefs)-1)
	}
}

func (m *Model) updateWorkspace(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.workspace.tab == wsTabPRs && m.workspace.prDetail != nil {
			return m.updateWorkspacePRs(msg)
		}
		if m.workspace.tab == wsTabIssues && m.workspace.issueDetail != nil {
			return m.updateWorkspaceIssues(msg)
		}
		if m.workspace.tab == wsTabCommits && m.workspace.commitDiffOpen {
			return m.updateWorkspaceCommits(msg)
		}
		m.workspace.seq++
		prev := m.workspace.prevState
		if prev == stateLoading || prev == stateError || prev == stateWorkspace {
			prev = stateDashboard
		}
		if prev == stateDashboard && len(m.repoList) == 0 {
			return m.goHome()
		}
		m.state = prev
		if prev == stateDashboard {
			m.recalculateLayout(m.width, m.height)
		}
		return nil

	case "tab":
		m.workspace.tab = (m.workspace.tab + 1) % 4
		return m.ensureActiveWorkspaceTabLoaded()
	case "shift+tab":
		m.workspace.tab = (m.workspace.tab + 3) % 4
		return m.ensureActiveWorkspaceTabLoaded()
	case "1":
		m.workspace.tab = wsTabCode
		return nil
	case "2":
		m.workspace.tab = wsTabPRs
		return m.ensureWorkspacePRsLoaded()
	case "3":
		m.workspace.tab = wsTabIssues
		return m.ensureWorkspaceIssuesLoaded()
	case "4":
		m.workspace.tab = wsTabCommits
		return m.ensureWorkspaceCommitsLoaded()

	case "s":
		if m.workspace.starUpdating {
			return nil
		}
		m.workspace.starUpdating = true
		m.workspace.starSeq++
		target := !m.workspace.starred
		return wsToggleStarCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.starSeq, m.workspace.owner, m.workspace.repo, target)

	case "Y":
		return m.copyWorkspaceCloneCommand()

	case "C":
		if m.workspace.cloning {
			return nil
		}
		m.workspace.cloning = true
		return tea.Batch(
			wsCloneRepoCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo),
			m.setTransientToast("⏳ Cloning "+m.workspace.owner+"/"+m.workspace.repo+"...", 4*time.Second),
		)

	case "b":
		m.workspace.showBranchModal = true
		m.workspace.branchFilter.Reset()
		m.workspace.branchFilter.Focus()
		m.workspace.applyBranchFilter()
		for i, r := range m.workspace.filteredRefs {
			if r.Name == m.workspace.activeRef {
				m.workspace.branchIndex = i
				break
			}
		}
		return textinput.Blink

	case "O":
		url := m.workspaceRepoURL()
		if url != "" {
			if err := utils.OpenBrowser(url); err != nil {
				return m.setTransientToast("❌ Failed to open browser", 2*time.Second)
			}
			return m.setTransientToast("🌐 Opened in browser", 2*time.Second)
		}
		return nil

	case "y":
		url := m.workspaceRepoURL()
		if url != "" {
			if err := utils.CopyToClipboard(url); err != nil {
				return m.setTransientToast("❌ Failed to copy URL", 2*time.Second)
			}
			return m.setTransientToast("📋 Copied repo URL!", 2*time.Second)
		}
		return nil
	}

	switch m.workspace.tab {
	case wsTabCode:
		return m.updateWorkspaceCode(msg)
	case wsTabPRs:
		return m.updateWorkspacePRs(msg)
	case wsTabIssues:
		return m.updateWorkspaceIssues(msg)
	case wsTabCommits:
		return m.updateWorkspaceCommits(msg)
	}
	return nil
}

func (m *Model) ensureActiveWorkspaceTabLoaded() tea.Cmd {
	switch m.workspace.tab {
	case wsTabPRs:
		return m.ensureWorkspacePRsLoaded()
	case wsTabIssues:
		return m.ensureWorkspaceIssuesLoaded()
	case wsTabCommits:
		return m.ensureWorkspaceCommitsLoaded()
	}
	return nil
}

func (m Model) workspaceRepoURL() string {
	if strings.TrimSpace(m.workspace.meta.HTMLURL) != "" {
		return m.workspace.meta.HTMLURL
	}
	if m.workspace.owner != "" && m.workspace.repo != "" {
		return "https://github.com/" + m.workspace.owner + "/" + m.workspace.repo
	}
	return ""
}

func (m Model) workspaceActiveRefOrHEAD() string {
	if strings.TrimSpace(m.workspace.activeRef) != "" {
		return m.workspace.activeRef
	}
	return "HEAD"
}

func (m Model) workspaceHeaderHeight(height int) int {
	if height >= 22 {
		return 5
	}
	return 4
}

func (m Model) workspaceTreeVisibleRows() int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}
	availH := max(6, h-m.workspaceHeaderHeight(h)-1)
	if w >= 95 {
		return max(1, availH-2)
	}
	treeH := max(5, availH*40/100)
	return max(1, treeH-2)
}

func (m Model) workspaceListVisibleRows() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	availH := max(6, h-m.workspaceHeaderHeight(h)-1)
	return max(1, availH-4)
}

func (m *Model) recalculateWorkspaceLayout(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	headerH := m.workspaceHeaderHeight(height)
	availH := max(6, height-headerH-1)
	if width >= 95 {
		treeW := max(26, min(42, width*30/100))
		viewerW := width - treeW
		m.workspace.viewer.Width = max(20, viewerW-4)
		m.workspace.viewer.Height = max(1, availH-2)
		diffFilesW := max(26, min(44, width*32/100))
		m.workspace.commitDiffViewport.Width = max(20, (width-diffFilesW)-4)
		m.workspace.commitDiffViewport.Height = max(1, availH-2)
	} else {
		treeH := max(5, availH*40/100)
		viewerH := max(4, availH-treeH)
		m.workspace.viewer.Width = max(20, width-4)
		m.workspace.viewer.Height = max(1, viewerH-2)
		m.workspace.commitDiffViewport.Width = max(20, width-4)
		m.workspace.commitDiffViewport.Height = max(1, viewerH-2)
	}
	m.workspace.prViewport.Width = max(20, width-4)
	m.workspace.prViewport.Height = max(1, availH-2)
	m.workspace.issueViewport.Width = max(20, width-4)
	m.workspace.issueViewport.Height = max(1, availH-2)
	if m.workspace.prDetail != nil {
		m.refreshPRDetailContent()
	}
	if m.workspace.issueDetail != nil {
		m.refreshIssueDetailContent()
	}
	if m.workspace.commitDiffOpen {
		m.refreshCommitDiffViewport()
	}
	if m.workspace.showCommentModal {
		m.workspace.commentInput.SetWidth(max(36, min(76, width-12)))
		m.workspace.commentInput.SetHeight(max(5, min(10, height-12)))
	}
	m.workspace.clampTreeCursor(m.workspaceTreeVisibleRows())
	m.workspace.clampPRCursor(m.workspaceListVisibleRows())
	m.workspace.clampIssueCursor(m.workspaceListVisibleRows())
	m.workspace.clampCommitCursor(m.workspaceListVisibleRows())
	m.workspace.clampCommitFileCursor(m.workspaceTreeVisibleRows())
}

func (m Model) renderWorkspaceView(width, height int) string {
	if m.workspace.showBranchModal {
		return m.renderWorkspaceBranchModal(width, height)
	}
	if m.workspace.showCommentModal {
		return m.renderWorkspaceCommentModal(width, height)
	}

	header := m.renderWorkspaceHeader(width, height)
	headerH := lipgloss.Height(header)
	availH := max(4, height-headerH-1)

	var body string
	switch m.workspace.tab {
	case wsTabCode:
		body = m.renderWorkspaceCodeTab(width, availH)
	case wsTabPRs:
		body = m.renderWorkspacePRsTab(width, availH)
	case wsTabIssues:
		body = m.renderWorkspaceIssuesTab(width, availH)
	case wsTabCommits:
		body = m.renderWorkspaceCommitsTab(width, availH)
	}

	statusBar := m.renderWorkspaceStatusBar(width)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

func (m Model) renderWorkspaceHeader(width, height int) string {
	headerH := m.workspaceHeaderHeight(height)
	innerW := max(10, width-4)

	crumbPrefix := "🏠"
	if m.workspace.breadcrumb != "" {
		crumbPrefix += " / " + m.workspace.breadcrumb
	}
	repoSlug := m.workspace.owner + "/" + m.workspace.repo
	refName := m.workspace.activeRef
	if refName == "" {
		refName = "HEAD"
	}

	crumbText := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(crumbPrefix+" / ") +
		lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render(repoSlug)
	refBadge := lipgloss.NewStyle().
		Foreground(m.theme.Background).
		Background(m.theme.Secondary).
		Bold(true).
		Padding(0, 1).
		Render("🌿 " + refName)

	tabDefs := []struct {
		id    workspaceTab
		label string
	}{
		{wsTabCode, "1:Code"},
		{wsTabPRs, "2:PRs"},
		{wsTabIssues, "3:Issues"},
		{wsTabCommits, "4:Commits"},
	}
	var tabParts []string
	for _, td := range tabDefs {
		if m.workspace.tab == td.id {
			tabParts = append(tabParts, lipgloss.NewStyle().
				Bold(true).
				Foreground(m.theme.Background).
				Background(m.theme.Primary).
				Padding(0, 1).
				Render(td.label))
		} else {
			tabParts = append(tabParts, lipgloss.NewStyle().
				Foreground(m.theme.Muted).
				Background(m.theme.Surface).
				Padding(0, 1).
				Render(td.label))
		}
	}
	tabsBlock := strings.Join(tabParts, " ")

	leftTop := crumbText + " " + refBadge
	var line1 string
	if lipgloss.Width(leftTop)+lipgloss.Width(tabsBlock)+2 <= innerW {
		gap := innerW - lipgloss.Width(leftTop) - lipgloss.Width(tabsBlock)
		line1 = leftTop + strings.Repeat(" ", gap) + tabsBlock
	} else {
		line1 = TruncateDisplayWidth(repoSlug+" ["+refName+"] "+tabsBlock, innerW, "…")
	}

	metaLine := "Loading repository metadata..."
	if m.workspace.metaErr != nil {
		metaLine = "⚠ " + m.workspace.metaErr.Error()
	} else if m.workspace.metaLoaded {
		mt := m.workspace.meta
		parts := []string{
			fmt.Sprintf("★ %d", mt.Stars),
			fmt.Sprintf("⑂ %d", mt.Forks),
			fmt.Sprintf("👁 %d", mt.Watchers),
		}
		if mt.License != "" {
			parts = append(parts, "⚖ "+mt.License)
		}
		if mt.Language != "" {
			parts = append(parts, "● "+mt.Language)
		}
		if len(mt.Topics) > 0 {
			maxTopics := min(4, len(mt.Topics))
			parts = append(parts, "#"+strings.Join(mt.Topics[:maxTopics], " #"))
		}
		metaLine = strings.Join(parts, "  ")
	}
	line2 := lipgloss.NewStyle().Foreground(m.theme.Accent).Render(TruncateDisplayWidth(metaLine, innerW, "…"))

	lines := []string{" " + line1, " " + line2}
	if headerH >= 5 {
		desc := strings.TrimSpace(m.workspace.meta.Description)
		if desc == "" && m.workspace.metaLoaded {
			desc = "No description provided."
		}
		line3 := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(TruncateDisplayWidth(desc, innerW, "…"))
		lines = append(lines, " "+line3)
	}

	return WrapInTitledBorder(strings.Join(lines, "\n"), width, headerH, "Repository Workspace", true, m.theme)
}

func (m Model) renderWorkspacePlaceholderTab(width, height int, title, message string) string {
	innerW := max(10, width-4)
	msg := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(TruncateDisplayWidth(message, innerW, "…"))
	return WrapInTitledBorder("\n "+msg, width, height, title, true, m.theme)
}

func (m Model) renderWorkspaceBranchModal(width, height int) string {
	boxW := min(60, max(36, width-6))
	innerW := boxW - 6

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Primary).Render("🌿 SWITCH BRANCH / TAG")
	inCopy := m.workspace.branchFilter
	inCopy.Width = max(16, innerW-4)

	maxRows := max(4, min(12, height-10))
	var rows []string
	if !m.workspace.refsLoaded {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("  Loading branches & tags..."))
	} else if len(m.workspace.filteredRefs) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("  No matching branches or tags."))
	} else {
		start := 0
		if m.workspace.branchIndex >= maxRows {
			start = m.workspace.branchIndex - maxRows + 1
		}
		end := min(len(m.workspace.filteredRefs), start+maxRows)
		for i := start; i < end; i++ {
			r := m.workspace.filteredRefs[i]
			kindTag := "branch"
			if r.Kind == "tag" {
				kindTag = "tag"
			}
			activeMark := "  "
			if r.Name == m.workspace.activeRef {
				activeMark = "✓ "
			}
			label := TruncateDisplayWidth(fmt.Sprintf("%s%-6s %s", activeMark, "["+kindTag+"]", r.Name), innerW, "…")
			if i == m.workspace.branchIndex {
				rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Render(padRightDisplay(label, innerW)))
			} else {
				rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Text).Render(label))
			}
		}
	}

	hint := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(TruncateDisplayWidth("[↑/↓] Select  [Enter] Switch  [Esc] Close", innerW, "…"))
	sections := []string{title, "", inCopy.View(), "", strings.Join(rows, "\n"), "", hint}

	card := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(boxW).
		MaxWidth(width).
		MaxHeight(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) renderWorkspaceStatusBar(width int) string {
	refName := m.workspaceActiveRefOrHEAD()
	repoPill := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Background).
		Background(m.theme.Primary).
		Padding(0, 1).
		Render(TruncateDisplayWidth(fmt.Sprintf("📦 %s/%s@%s", m.workspace.owner, m.workspace.repo, refName), max(14, width/3), "…"))

	var toastPill string
	if m.toastMessage != "" {
		toastPill = lipgloss.NewStyle().
			Foreground(m.theme.Background).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1).
			Render(m.toastMessage)
	}

	themePill := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Padding(0, 1).
		Render("🎨 " + m.theme.ID)

	var rateLimitPill string
	rlInfo := ghclient.GetRateLimitInfo()
	if text, isWarn, isCrit := rlInfo.FormatRateLimitIndicator(); text != "" {
		rlStyle := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.Surface).Padding(0, 1)
		if isCrit {
			rlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(lipgloss.Color("#f7768e")).Bold(true).Padding(0, 1)
		} else if isWarn {
			rlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(lipgloss.Color("#e0af68")).Bold(true).Padding(0, 1)
		}
		rateLimitPill = rlStyle.Render(text)
	}

	items := []string{repoPill}
	if toastPill != "" {
		items = append(items, " ", toastPill)
	}
	items = append(items, " ", themePill)
	if rateLimitPill != "" {
		items = append(items, " ", rateLimitPill)
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, items...)

	keyStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	bullet := lipgloss.NewStyle().Foreground(m.theme.Border).Render(" • ")
	kb := func(k, d string) string { return keyStyle.Render(k) + " " + descStyle.Render(d) }

	var helpItems []string
	if width >= 120 {
		helpItems = []string{kb("Tab", "tab"), kb("b", "branch"), kb("←/→", "pane"), kb("R", "readme"), kb("Esc", "back"), kb("?", "help"), kb("q", "quit")}
	} else if width >= 80 {
		helpItems = []string{kb("Tab", "tab"), kb("b", "branch"), kb("Esc", "back"), kb("?", "help"), kb("q", "quit")}
	} else {
		helpItems = []string{kb("Esc", "back"), kb("q", "quit")}
	}
	helpText := strings.Join(helpItems, bullet)

	gap := width - lipgloss.Width(left) - lipgloss.Width(helpText)
	if gap < 1 {
		leftItems := []string{repoPill}
		if toastPill != "" {
			leftItems = append(leftItems, " ", toastPill)
		}
		left = lipgloss.JoinHorizontal(lipgloss.Top, leftItems...)
		gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
	}
	if gap < 1 {
		left = repoPill
		gap = max(1, width-lipgloss.Width(left)-lipgloss.Width(helpText))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", gap), helpText)
	return lipgloss.NewStyle().
		Width(width).
		Height(1).
		MaxHeight(1).
		Background(m.theme.Background).
		Render(bar)
}
