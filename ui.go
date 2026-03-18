package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Dashboard is the main TUI application.
type Dashboard struct {
	config    *Config
	beadsDirs []string

	app *tview.Application

	// Panels
	header       *tview.TextView
	reposList    *tview.List
	statsView    *tview.TextView
	priorityView *tview.TextView
	labelsView   *tview.TextView
	issuesTable  *tview.Table
	detailView   *tview.TextView
	blockedTable *tview.Table
	epicsView    *tview.TextView
	daemonsView  *tview.TextView
	footer       *tview.TextView

	// Data
	data *AggregateData
	mu   sync.RWMutex

	// State
	selectedRepo int
	focusIndex   int
	focusables   []tview.Primitive

	// Lifecycle
	stopChan chan struct{}
}

// NewDashboard creates and wires the entire TUI.
func NewDashboard(cfg *Config, beadsDirs []string) *Dashboard {
	d := &Dashboard{
		config:    cfg,
		beadsDirs: beadsDirs,
		app:       tview.NewApplication(),
		stopChan:  make(chan struct{}),
	}
	d.buildUI()
	return d
}

// Run starts polling and enters the tview event loop (blocks).
func (d *Dashboard) Run() error {
	go d.startPolling()
	err := d.app.Run()
	close(d.stopChan)
	return err
}

// -------------------------------------------------------------------------
// UI construction
// -------------------------------------------------------------------------

func (d *Dashboard) buildUI() {
	// Header
	d.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	// Repos list (left sidebar, top)
	d.reposList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.NewRGBColor(40, 50, 65))
	d.reposList.SetBorder(true).
		SetTitle(" Repositories ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Aggregate stats
	d.statsView = tview.NewTextView().SetDynamicColors(true)
	d.statsView.SetBorder(true).
		SetTitle(" Aggregate ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Priority distribution
	d.priorityView = tview.NewTextView().SetDynamicColors(true)
	d.priorityView.SetBorder(true).
		SetTitle(" Priority ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Labels
	d.labelsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.labelsView.SetBorder(true).
		SetTitle(" Labels ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Issues table (right, top)
	d.issuesTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	d.issuesTable.SetBorder(true).
		SetTitle(" Issues ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)
	d.issuesTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(40, 50, 65)).
		Foreground(tcell.ColorWhite))

	// Detail
	d.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true)
	d.detailView.SetBorder(true).
		SetTitle(" Detail ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Blocked table
	d.blockedTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	d.blockedTable.SetBorder(true).
		SetTitle(" Blocked ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)
	d.blockedTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(40, 50, 65)).
		Foreground(tcell.ColorWhite))

	// Epics
	d.epicsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.epicsView.SetBorder(true).
		SetTitle(" Epics ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Daemons
	d.daemonsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.daemonsView.SetBorder(true).
		SetTitle(" Daemons ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDimGray)

	// Footer
	d.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Initial placeholder content
	d.header.SetText(" [yellow::b]BEADS-PANE[-::-] [white]◆[-] Agent Control Pane         [dim]loading...[-]")
	d.footer.SetText(" [dim]q[-]:quit  [dim]r[-]:refresh  [dim]Tab[-]:next pane  [dim]↑↓[-]:navigate  [dim]Enter[-]:select")
	d.detailView.SetText(" [dim]Select an issue to view details[-]")

	// --- Layout -----------------------------------------------------------

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.reposList, 0, 3, true).
		AddItem(d.statsView, 9, 0, false).
		AddItem(d.priorityView, 7, 0, false).
		AddItem(d.labelsView, 0, 2, false)

	bottomRight := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(d.epicsView, 0, 1, false).
		AddItem(d.daemonsView, 0, 1, false)

	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.issuesTable, 0, 3, false).
		AddItem(d.detailView, 0, 2, false).
		AddItem(d.blockedTable, 0, 2, false).
		AddItem(bottomRight, 0, 1, false)

	mainArea := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftPanel, 34, 0, true).
		AddItem(rightPanel, 0, 1, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.header, 1, 0, false).
		AddItem(mainArea, 0, 1, true).
		AddItem(d.footer, 1, 0, false)

	// --- Focus cycling ----------------------------------------------------

	d.focusables = []tview.Primitive{d.reposList, d.issuesTable, d.blockedTable}
	d.focusIndex = 0

	// --- Callbacks --------------------------------------------------------

	d.reposList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		d.selectedRepo = index
		d.mu.RLock()
		defer d.mu.RUnlock()
		d.refreshIssuesTable()
		d.setDetailText(-1)
	})

	d.issuesTable.SetSelectionChangedFunc(func(row, _ int) {
		if row > 0 {
			d.mu.RLock()
			defer d.mu.RUnlock()
			d.setDetailText(row - 1)
		}
	})

	// --- Global keys ------------------------------------------------------

	d.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			d.focusIndex = (d.focusIndex + 1) % len(d.focusables)
			d.app.SetFocus(d.focusables[d.focusIndex])
			return nil
		case tcell.KeyBacktab:
			d.focusIndex = (d.focusIndex - 1 + len(d.focusables)) % len(d.focusables)
			d.app.SetFocus(d.focusables[d.focusIndex])
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				d.app.Stop()
				return nil
			case 'r':
				go d.poll()
				return nil
			}
		}
		return event
	})

	d.app.SetRoot(root, true)
}

// -------------------------------------------------------------------------
// Polling
// -------------------------------------------------------------------------

func (d *Dashboard) startPolling() {
	d.poll()

	ticker := time.NewTicker(time.Duration(d.config.PollIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.poll()
		case <-d.stopChan:
			return
		}
	}
}

func (d *Dashboard) poll() {
	data := pollAllRepos(d.beadsDirs)

	d.mu.Lock()
	d.data = data
	d.mu.Unlock()

	d.app.QueueUpdateDraw(func() {
		d.mu.RLock()
		defer d.mu.RUnlock()
		if d.data == nil {
			return
		}
		d.updateHeader()
		d.updateReposList()
		d.updateStatsView()
		d.updatePriorityView()
		d.updateLabelsView()
		d.refreshIssuesTable()
		d.updateBlockedTable()
		d.updateEpicsView()
		d.updateDaemonsView()
	})
}

// -------------------------------------------------------------------------
// Panel renderers (caller must hold at least mu.RLock)
// -------------------------------------------------------------------------

func (d *Dashboard) updateHeader() {
	now := d.data.LastPoll.Format("15:04:05")
	d.header.SetText(fmt.Sprintf(
		" [yellow::b]BEADS-PANE[-::-] [white]◆[-] Agent Control Pane       [dim]%d repos │ poll %ds │ %s[-]",
		len(d.data.Repos), d.config.PollIntervalSec, now,
	))
}

func (d *Dashboard) updateReposList() {
	prev := d.reposList.GetCurrentItem()
	d.reposList.Clear()

	for _, repo := range d.data.Repos {
		total, active := 0, 0
		if repo.Stats != nil {
			total = repo.Stats.TotalIssues
			active = repo.Stats.OpenIssues + repo.Stats.InProgressIssues
		}
		main := fmt.Sprintf(" %s", repo.Name)
		var sec string
		if repo.Error != nil {
			sec = "   [red]error[-]"
		} else {
			sec = fmt.Sprintf("   [dim]%d total, %d active[-]", total, active)
		}
		d.reposList.AddItem(main, sec, 0, nil)
	}

	if prev >= 0 && prev < d.reposList.GetItemCount() {
		d.reposList.SetCurrentItem(prev)
	}
}

func (d *Dashboard) updateStatsView() {
	a := d.data
	var b strings.Builder
	fmt.Fprintf(&b, " [white::b]Total[-::-]       %d\n", a.TotalIssues)
	fmt.Fprintf(&b, " [green]Open[-]        %d\n", a.TotalOpen)
	fmt.Fprintf(&b, " [#5f87ff]In Prog[-]     %d\n", a.TotalInProgress)
	fmt.Fprintf(&b, " [red]Blocked[-]     %d\n", a.TotalBlocked)
	fmt.Fprintf(&b, " [#888888]Closed[-]      %d\n", a.TotalClosed)
	fmt.Fprintf(&b, " [green::b]Ready[-::-]       %d\n", a.TotalReady)
	fmt.Fprintf(&b, " [dim]Lead Time  %s avg[-]", formatHours(a.AvgLeadTime))
	d.statsView.SetText(b.String())
}

func (d *Dashboard) updatePriorityView() {
	var b strings.Builder
	labels := [5]string{"Critical", "High", "Medium", "Low", "Backlog"}
	colors := [5]string{"red", "#ff8700", "yellow", "green", "#888888"}
	markers := [5]string{"■", "■", "■", "■", "□"}

	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, " [%s]%s[-] %-9s [white]%3d[-]\n",
			colors[i], markers[i], labels[i], d.data.PriorityDist[i])
	}
	d.priorityView.SetText(b.String())
}

func (d *Dashboard) updateLabelsView() {
	var b strings.Builder
	max := 14
	if len(d.data.AllLabels) < max {
		max = len(d.data.AllLabels)
	}
	for i := 0; i < max; i++ {
		lc := d.data.AllLabels[i]
		fmt.Fprintf(&b, " [#87afaf]%-15s[-] [white]%3d[-]\n", truncate(lc.Label, 15), lc.Count)
	}
	if len(d.data.AllLabels) > max {
		fmt.Fprintf(&b, " [dim]... %d more[-]", len(d.data.AllLabels)-max)
	}
	d.labelsView.SetText(b.String())
}

func (d *Dashboard) refreshIssuesTable() {
	d.issuesTable.Clear()

	// Header row
	for i, h := range []string{"ID", "Title", "Status", "Pri", "Type"} {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		if i == 1 {
			cell.SetExpansion(1)
		} else {
			cell.SetMaxWidth(20)
		}
		d.issuesTable.SetCell(0, i, cell)
	}

	if d.data == nil || len(d.data.Repos) == 0 {
		return
	}

	idx := d.selectedRepo
	if idx >= len(d.data.Repos) {
		idx = 0
	}
	repo := d.data.Repos[idx]
	total := 0
	if repo.Stats != nil {
		total = repo.Stats.TotalIssues
	}
	d.issuesTable.SetTitle(fmt.Sprintf(" Issues · %s [%d] ", repo.Name, total))

	row := 1
	for _, issue := range repo.Issues {
		if issue.Status == "closed" {
			continue
		}

		d.issuesTable.SetCell(row, 0,
			tview.NewTableCell(truncate(issue.ID, 20)).SetMaxWidth(20).
				SetTextColor(tcell.ColorWhite))

		d.issuesTable.SetCell(row, 1,
			tview.NewTableCell(truncate(issue.Title, 50)).SetExpansion(1).
				SetTextColor(tcell.ColorWhite))

		stCell := tview.NewTableCell(statusLabel(issue.Status)).SetMaxWidth(8)
		switch issue.Status {
		case "open":
			stCell.SetTextColor(tcell.ColorGreen)
		case "in_progress":
			stCell.SetTextColor(tcell.NewRGBColor(95, 135, 255))
		case "blocked":
			stCell.SetTextColor(tcell.ColorRed)
		}
		d.issuesTable.SetCell(row, 2, stCell)

		prCell := tview.NewTableCell(priorityName(issue.Priority)).SetMaxWidth(4)
		switch issue.Priority {
		case 0:
			prCell.SetTextColor(tcell.ColorRed)
		case 1:
			prCell.SetTextColor(tcell.NewRGBColor(255, 135, 0))
		case 2:
			prCell.SetTextColor(tcell.ColorYellow)
		case 3:
			prCell.SetTextColor(tcell.ColorGreen)
		default:
			prCell.SetTextColor(tcell.ColorGray)
		}
		d.issuesTable.SetCell(row, 3, prCell)

		d.issuesTable.SetCell(row, 4,
			tview.NewTableCell(typeLabel(issue.IssueType)).SetMaxWidth(6).
				SetTextColor(tcell.ColorWhite))

		row++
	}

	if row == 1 {
		d.issuesTable.SetCell(1, 0,
			tview.NewTableCell("[dim]No active issues[-]").SetSelectable(false).SetExpansion(1))
	}
}

func (d *Dashboard) setDetailText(issueIdx int) {
	if d.data == nil || len(d.data.Repos) == 0 {
		d.detailView.SetText(" [dim]Select an issue to view details[-]")
		return
	}
	idx := d.selectedRepo
	if idx >= len(d.data.Repos) {
		return
	}
	repo := d.data.Repos[idx]

	var active []Issue
	for _, iss := range repo.Issues {
		if iss.Status != "closed" {
			active = append(active, iss)
		}
	}

	if issueIdx < 0 || issueIdx >= len(active) {
		d.detailView.SetText(" [dim]Select an issue to view details[-]")
		return
	}

	iss := active[issueIdx]
	var b strings.Builder
	fmt.Fprintf(&b, " [yellow::b]%s[-::-] [white]◆[-] %s\n\n", iss.ID, iss.Title)
	fmt.Fprintf(&b, " [%s]%s[-]", statusColor(iss.Status), statusLabel(iss.Status))
	fmt.Fprintf(&b, "  │  [%s]%s %s[-]", priorityColor(iss.Priority), priorityName(iss.Priority), priorityLabel(iss.Priority))
	fmt.Fprintf(&b, "  │  %s\n", iss.IssueType)
	fmt.Fprintf(&b, " Created: %s  │  Updated: %s\n", parseShortDate(iss.CreatedAt), parseShortDate(iss.UpdatedAt))

	if iss.DependencyCount > 0 || iss.DependentCount > 0 {
		fmt.Fprintf(&b, " Deps: %d  │  Dependents: %d\n", iss.DependencyCount, iss.DependentCount)
	}
	if iss.Assignee != "" {
		fmt.Fprintf(&b, " Assignee: %s\n", iss.Assignee)
	}
	if len(iss.Labels) > 0 {
		fmt.Fprintf(&b, " Labels: %s\n", strings.Join(iss.Labels, ", "))
	}
	if len(iss.BlockedBy) > 0 {
		fmt.Fprintf(&b, " [red]Blocked by: %s[-]\n", strings.Join(iss.BlockedBy, ", "))
	}

	desc := strings.TrimSpace(iss.Description)
	if desc != "" {
		fmt.Fprintf(&b, "\n %s", truncate(desc, 600))
	}
	d.detailView.SetText(b.String())
	d.detailView.ScrollToBeginning()
}

func (d *Dashboard) updateBlockedTable() {
	d.blockedTable.Clear()

	headers := []string{"Repo", "ID", "Title", "Blocked By"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		if i == 2 {
			cell.SetExpansion(1)
		} else {
			cell.SetMaxWidth(20)
		}
		d.blockedTable.SetCell(0, i, cell)
	}

	d.blockedTable.SetTitle(fmt.Sprintf(" Blocked [%d] ", len(d.data.AllBlocked)))

	for i, bi := range d.data.AllBlocked {
		row := i + 1
		d.blockedTable.SetCell(row, 0,
			tview.NewTableCell(truncate(bi.RepoName, 14)).SetMaxWidth(14).
				SetTextColor(tcell.NewRGBColor(135, 175, 175)))
		d.blockedTable.SetCell(row, 1,
			tview.NewTableCell(truncate(bi.Issue.ID, 20)).SetMaxWidth(20).
				SetTextColor(tcell.ColorWhite))
		d.blockedTable.SetCell(row, 2,
			tview.NewTableCell(truncate(bi.Issue.Title, 40)).SetExpansion(1).
				SetTextColor(tcell.ColorWhite))

		blockers := strings.Join(bi.Issue.BlockedBy, ", ")
		d.blockedTable.SetCell(row, 3,
			tview.NewTableCell(truncate(blockers, 30)).SetMaxWidth(30).
				SetTextColor(tcell.ColorRed))
	}

	if len(d.data.AllBlocked) == 0 {
		d.blockedTable.SetCell(1, 0,
			tview.NewTableCell("[dim]No blocked issues[-]").SetSelectable(false).SetExpansion(1))
	}
}

func (d *Dashboard) updateEpicsView() {
	var b strings.Builder
	if len(d.data.AllEpics) == 0 {
		b.WriteString(" [dim]No epics[-]")
	}

	for _, ew := range d.data.AllEpics {
		pct := 0
		if ew.TotalChildren > 0 {
			pct = ew.ClosedChildren * 100 / ew.TotalChildren
		}
		bar := progressBar(pct, 20)
		fmt.Fprintf(&b, " [#87afaf]%s[-]\n", ew.RepoName)
		fmt.Fprintf(&b, " [yellow]%s[-] %s\n", ew.Epic.ID, truncate(ew.Epic.Title, 40))
		fmt.Fprintf(&b, " %s  %d/%d (%d%%)\n\n", bar, ew.ClosedChildren, ew.TotalChildren, pct)
	}
	d.epicsView.SetText(b.String())
}

func (d *Dashboard) updateDaemonsView() {
	var b strings.Builder
	if len(d.data.Daemons) == 0 {
		b.WriteString(" [dim]No daemons running[-]")
	}

	for _, dm := range d.data.Daemons {
		name := filepath.Base(dm.WorkspacePath)
		alive := "[red]✗[-]"
		if dm.Alive {
			alive = "[green]✓[-]"
		}
		fmt.Fprintf(&b, " %s [white]%s[-]  PID %d  %s  v%s\n",
			alive, name, dm.PID, formatUptime(dm.UptimeSeconds), dm.Version)
	}
	d.daemonsView.SetText(b.String())
}

func progressBar(pct int, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	empty := width - filled
	return "[green]" + strings.Repeat("█", filled) + "[-]" +
		"[dim]" + strings.Repeat("░", empty) + "[-]"
}
