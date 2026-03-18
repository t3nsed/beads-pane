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

type Dashboard struct {
	config    *Config
	beadsDirs []string

	app   *tview.Application
	theme Theme

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

	data *AggregateData
	mu   sync.RWMutex

	selectedRepo int
	focusIndex   int
	focusables   []tview.Primitive

	stopChan chan struct{}
}

func NewDashboard(cfg *Config, beadsDirs []string, themeName string) *Dashboard {
	d := &Dashboard{
		config:    cfg,
		beadsDirs: beadsDirs,
		app:       tview.NewApplication(),
		stopChan:  make(chan struct{}),
	}
	if themeName == "dark" {
		d.theme = newDarkTheme()
	} else {
		d.theme = newLightTheme()
	}
	d.buildUI()
	return d
}

func (d *Dashboard) Run() error {
	go d.startPolling()
	err := d.app.Run()
	close(d.stopChan)
	return err
}

func (d *Dashboard) buildUI() {
	d.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	d.reposList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true)
	d.reposList.SetBorder(true).
		SetTitle(" Repositories ").
		SetTitleAlign(tview.AlignLeft)

	d.statsView = tview.NewTextView().SetDynamicColors(true)
	d.statsView.SetBorder(true).
		SetTitle(" Aggregate ").
		SetTitleAlign(tview.AlignLeft)

	d.priorityView = tview.NewTextView().SetDynamicColors(true)
	d.priorityView.SetBorder(true).
		SetTitle(" Priority ").
		SetTitleAlign(tview.AlignLeft)

	d.labelsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.labelsView.SetBorder(true).
		SetTitle(" Labels ").
		SetTitleAlign(tview.AlignLeft)

	d.issuesTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	d.issuesTable.SetBorder(true).
		SetTitle(" Issues ").
		SetTitleAlign(tview.AlignLeft)

	d.detailView = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true)
	d.detailView.SetBorder(true).
		SetTitle(" Detail ").
		SetTitleAlign(tview.AlignLeft)

	d.blockedTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	d.blockedTable.SetBorder(true).
		SetTitle(" Blocked ").
		SetTitleAlign(tview.AlignLeft)

	d.epicsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.epicsView.SetBorder(true).
		SetTitle(" Epics ").
		SetTitleAlign(tview.AlignLeft)

	d.daemonsView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	d.daemonsView.SetBorder(true).
		SetTitle(" Daemons ").
		SetTitleAlign(tview.AlignLeft)

	d.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	d.applyTheme()

	t := &d.theme
	d.header.SetText(fmt.Sprintf(
		" [%s::b]BEADS-PANE[-::-] [%s]◆[-] Agent Control Pane         [%s]loading...[-]",
		t.HeaderTag, t.FgTag, t.DimTag))
	d.footer.SetText(fmt.Sprintf(
		" [%s]q[-]:quit  [%s]r[-]:refresh  [%s]t[-]:theme  [%s]Tab[-]:pane  [%s]↑↓[-]:nav",
		t.DimTag, t.DimTag, t.DimTag, t.DimTag, t.DimTag))
	d.detailView.SetText(fmt.Sprintf(" [%s]Select an issue to view details[-]", t.DimTag))

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

	d.focusables = []tview.Primitive{d.reposList, d.issuesTable, d.blockedTable}
	d.focusIndex = 0

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
			case 't':
				d.toggleTheme()
				return nil
			}
		}
		return event
	})

	d.app.SetRoot(root, true)
}

func (d *Dashboard) applyTheme() {
	t := &d.theme

	for _, tv := range []*tview.TextView{
		d.header, d.statsView, d.priorityView, d.labelsView,
		d.detailView, d.epicsView, d.daemonsView, d.footer,
	} {
		tv.SetBackgroundColor(t.Bg)
		tv.SetTextColor(t.Fg)
		tv.SetBorderColor(t.Border)
		tv.SetTitleColor(t.Fg)
	}

	d.reposList.SetBackgroundColor(t.Bg)
	d.reposList.SetMainTextColor(t.Fg)
	d.reposList.SetSecondaryTextColor(t.Dim)
	d.reposList.SetSelectedBackgroundColor(t.SelectBg)
	d.reposList.SetSelectedTextColor(t.Fg)
	d.reposList.SetBorderColor(t.Border)
	d.reposList.SetTitleColor(t.Fg)

	selStyle := tcell.StyleDefault.Background(t.SelectBg).Foreground(t.Fg)
	for _, tbl := range []*tview.Table{d.issuesTable, d.blockedTable} {
		tbl.SetBackgroundColor(t.Bg)
		tbl.SetBorderColor(t.Border)
		tbl.SetTitleColor(t.Fg)
		tbl.SetSelectedStyle(selStyle)
	}
}

func (d *Dashboard) toggleTheme() {
	if d.theme.IsDark {
		d.theme = newLightTheme()
	} else {
		d.theme = newDarkTheme()
	}
	d.applyTheme()

	t := &d.theme
	d.footer.SetText(fmt.Sprintf(
		" [%s]q[-]:quit  [%s]r[-]:refresh  [%s]t[-]:theme  [%s]Tab[-]:pane  [%s]↑↓[-]:nav",
		t.DimTag, t.DimTag, t.DimTag, t.DimTag, t.DimTag))

	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.data != nil {
		d.updateHeader()
		d.updateReposList()
		d.updateStatsView()
		d.updatePriorityView()
		d.updateLabelsView()
		d.refreshIssuesTable()
		d.setDetailText(-1)
		d.updateBlockedTable()
		d.updateEpicsView()
		d.updateDaemonsView()
	}
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
// Panel renderers (caller must hold mu.RLock)
// -------------------------------------------------------------------------

func (d *Dashboard) updateHeader() {
	t := &d.theme
	now := d.data.LastPoll.Format("15:04:05")
	mode := "light"
	if t.IsDark {
		mode = "dark"
	}
	d.header.SetText(fmt.Sprintf(
		" [%s::b]BEADS-PANE[-::-] [%s]◆[-] Agent Control Pane       [%s]%d repos │ poll %ds │ %s │ %s[-]",
		t.HeaderTag, t.FgTag, t.DimTag, len(d.data.Repos), d.config.PollIntervalSec, mode, now,
	))
}

func (d *Dashboard) updateReposList() {
	t := &d.theme
	prev := d.reposList.GetCurrentItem()
	d.reposList.Clear()

	for _, repo := range d.data.Repos {
		total, active := 0, 0
		if repo.Stats != nil {
			total = repo.Stats.TotalIssues
			active = repo.Stats.OpenIssues + repo.Stats.InProgressIssues
		}
		mainText := fmt.Sprintf(" %s", repo.Name)
		var sec string
		if repo.Error != nil {
			sec = fmt.Sprintf("   [%s]error[-]", t.BlockedTag)
		} else {
			sec = fmt.Sprintf("   [%s]%d total, %d active[-]", t.DimTag, total, active)
		}
		d.reposList.AddItem(mainText, sec, 0, nil)
	}

	if prev >= 0 && prev < d.reposList.GetItemCount() {
		d.reposList.SetCurrentItem(prev)
	}
}

func (d *Dashboard) updateStatsView() {
	t := &d.theme
	a := d.data
	var b strings.Builder
	fmt.Fprintf(&b, " [%s::b]Total[-::-]       %d\n", t.FgTag, a.TotalIssues)
	fmt.Fprintf(&b, " [%s]Open[-]        %d\n", t.OpenTag, a.TotalOpen)
	fmt.Fprintf(&b, " [%s]In Prog[-]     %d\n", t.InProgTag, a.TotalInProgress)
	fmt.Fprintf(&b, " [%s]Blocked[-]     %d\n", t.BlockedTag, a.TotalBlocked)
	fmt.Fprintf(&b, " [%s]Closed[-]      %d\n", t.ClosedTag, a.TotalClosed)
	fmt.Fprintf(&b, " [%s::b]Ready[-::-]       %d\n", t.ReadyTag, a.TotalReady)
	fmt.Fprintf(&b, " [%s]Lead Time  %s avg[-]", t.DimTag, formatHours(a.AvgLeadTime))
	d.statsView.SetText(b.String())
}

func (d *Dashboard) updatePriorityView() {
	t := &d.theme
	var b strings.Builder
	labels := [5]string{"Critical", "High", "Medium", "Low", "Backlog"}
	markers := [5]string{"■", "■", "■", "■", "□"}

	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, " [%s]%s[-] %-9s [%s]%3d[-]\n",
			t.PriTag[i], markers[i], labels[i], t.FgTag, d.data.PriorityDist[i])
	}
	d.priorityView.SetText(b.String())
}

func (d *Dashboard) updateLabelsView() {
	t := &d.theme
	var b strings.Builder
	max := 14
	if len(d.data.AllLabels) < max {
		max = len(d.data.AllLabels)
	}
	for i := 0; i < max; i++ {
		lc := d.data.AllLabels[i]
		fmt.Fprintf(&b, " [%s]%-15s[-] [%s]%3d[-]\n", t.AccentTag, truncate(lc.Label, 15), t.FgTag, lc.Count)
	}
	if len(d.data.AllLabels) > max {
		fmt.Fprintf(&b, " [%s]... %d more[-]", t.DimTag, len(d.data.AllLabels)-max)
	}
	d.labelsView.SetText(b.String())
}

func (d *Dashboard) refreshIssuesTable() {
	t := &d.theme
	d.issuesTable.Clear()

	for i, h := range []string{"ID", "Title", "Status", "Pri", "Type"} {
		cell := tview.NewTableCell(h).
			SetTextColor(t.Header).
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
			tview.NewTableCell(truncate(issue.ID, 20)).SetMaxWidth(20).SetTextColor(t.Fg))
		d.issuesTable.SetCell(row, 1,
			tview.NewTableCell(truncate(issue.Title, 50)).SetExpansion(1).SetTextColor(t.Fg))
		d.issuesTable.SetCell(row, 2,
			tview.NewTableCell(statusLabel(issue.Status)).SetMaxWidth(8).
				SetTextColor(t.StatusColor(issue.Status)))
		d.issuesTable.SetCell(row, 3,
			tview.NewTableCell(priorityName(issue.Priority)).SetMaxWidth(4).
				SetTextColor(t.PriColor(issue.Priority)))
		d.issuesTable.SetCell(row, 4,
			tview.NewTableCell(typeLabel(issue.IssueType)).SetMaxWidth(6).SetTextColor(t.Fg))

		row++
	}

	if row == 1 {
		d.issuesTable.SetCell(1, 0,
			tview.NewTableCell(fmt.Sprintf("[%s]No active issues[-]", t.DimTag)).
				SetSelectable(false).SetExpansion(1))
	}
}

func (d *Dashboard) setDetailText(issueIdx int) {
	t := &d.theme
	if d.data == nil || len(d.data.Repos) == 0 {
		d.detailView.SetText(fmt.Sprintf(" [%s]Select an issue to view details[-]", t.DimTag))
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
		d.detailView.SetText(fmt.Sprintf(" [%s]Select an issue to view details[-]", t.DimTag))
		return
	}

	iss := active[issueIdx]
	var b strings.Builder
	fmt.Fprintf(&b, " [%s::b]%s[-::-] [%s]◆[-] %s\n\n", t.HeaderTag, iss.ID, t.FgTag, iss.Title)
	fmt.Fprintf(&b, " [%s]%s[-]", t.StatusTag(iss.Status), statusLabel(iss.Status))
	pri := iss.Priority
	if pri >= 0 && pri <= 4 {
		fmt.Fprintf(&b, "  │  [%s]%s %s[-]", t.PriTag[pri], priorityName(pri), priorityLabel(pri))
	}
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
		fmt.Fprintf(&b, " [%s]Blocked by: %s[-]\n", t.BlockedTag, strings.Join(iss.BlockedBy, ", "))
	}

	desc := strings.TrimSpace(iss.Description)
	if desc != "" {
		fmt.Fprintf(&b, "\n %s", truncate(desc, 600))
	}
	d.detailView.SetText(b.String())
	d.detailView.ScrollToBeginning()
}

func (d *Dashboard) updateBlockedTable() {
	t := &d.theme
	d.blockedTable.Clear()

	for i, h := range []string{"Repo", "ID", "Title", "Blocked By"} {
		cell := tview.NewTableCell(h).
			SetTextColor(t.Header).
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
			tview.NewTableCell(truncate(bi.RepoName, 14)).SetMaxWidth(14).SetTextColor(t.Accent))
		d.blockedTable.SetCell(row, 1,
			tview.NewTableCell(truncate(bi.Issue.ID, 20)).SetMaxWidth(20).SetTextColor(t.Fg))
		d.blockedTable.SetCell(row, 2,
			tview.NewTableCell(truncate(bi.Issue.Title, 40)).SetExpansion(1).SetTextColor(t.Fg))
		blockers := strings.Join(bi.Issue.BlockedBy, ", ")
		d.blockedTable.SetCell(row, 3,
			tview.NewTableCell(truncate(blockers, 30)).SetMaxWidth(30).SetTextColor(t.Blocked))
	}

	if len(d.data.AllBlocked) == 0 {
		d.blockedTable.SetCell(1, 0,
			tview.NewTableCell(fmt.Sprintf("[%s]No blocked issues[-]", t.DimTag)).
				SetSelectable(false).SetExpansion(1))
	}
}

func (d *Dashboard) updateEpicsView() {
	t := &d.theme
	var b strings.Builder
	if len(d.data.AllEpics) == 0 {
		fmt.Fprintf(&b, " [%s]No epics[-]", t.DimTag)
	}

	for _, ew := range d.data.AllEpics {
		pct := 0
		if ew.TotalChildren > 0 {
			pct = ew.ClosedChildren * 100 / ew.TotalChildren
		}
		bar := d.progressBar(pct, 20)
		fmt.Fprintf(&b, " [%s]%s[-]\n", t.AccentTag, ew.RepoName)
		fmt.Fprintf(&b, " [%s]%s[-] %s\n", t.HeaderTag, ew.Epic.ID, truncate(ew.Epic.Title, 40))
		fmt.Fprintf(&b, " %s  %d/%d (%d%%)\n\n", bar, ew.ClosedChildren, ew.TotalChildren, pct)
	}
	d.epicsView.SetText(b.String())
}

func (d *Dashboard) updateDaemonsView() {
	t := &d.theme
	var b strings.Builder
	if len(d.data.Daemons) == 0 {
		fmt.Fprintf(&b, " [%s]No daemons running[-]", t.DimTag)
	}

	for _, dm := range d.data.Daemons {
		name := filepath.Base(dm.WorkspacePath)
		alive := fmt.Sprintf("[%s]✗[-]", t.BlockedTag)
		if dm.Alive {
			alive = fmt.Sprintf("[%s]✓[-]", t.OpenTag)
		}
		fmt.Fprintf(&b, " %s [%s]%s[-]  PID %d  %s  v%s\n",
			alive, t.FgTag, name, dm.PID, formatUptime(dm.UptimeSeconds), dm.Version)
	}
	d.daemonsView.SetText(b.String())
}

func (d *Dashboard) progressBar(pct int, width int) string {
	t := &d.theme
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	empty := width - filled
	return "[" + t.ReadyTag + "]" + strings.Repeat("█", filled) + "[-]" +
		"[" + t.DimTag + "]" + strings.Repeat("░", empty) + "[-]"
}
