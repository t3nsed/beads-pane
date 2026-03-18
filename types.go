package main

import (
	"fmt"
	"time"
)

// Issue represents a beads issue from bd list --json.
type Issue struct {
	ID              string   `json:"id"`
	ContentHash     string   `json:"content_hash,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	AcceptCriteria  string   `json:"acceptance_criteria,omitempty"`
	Status          string   `json:"status"`
	Priority        int      `json:"priority"`
	IssueType       string   `json:"issue_type"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	ClosedAt        string   `json:"closed_at,omitempty"`
	SourceRepo      string   `json:"source_repo,omitempty"`
	DependencyCount int      `json:"dependency_count,omitempty"`
	DependentCount  int      `json:"dependent_count,omitempty"`
	BlockedByCount  int      `json:"blocked_by_count,omitempty"`
	BlockedBy       []string `json:"blocked_by,omitempty"`
	Assignee        string   `json:"assignee,omitempty"`
	Labels          []string `json:"labels,omitempty"`
}

// Stats from bd stats --json.
type Stats struct {
	TotalIssues             int     `json:"total_issues"`
	OpenIssues              int     `json:"open_issues"`
	InProgressIssues        int     `json:"in_progress_issues"`
	ClosedIssues            int     `json:"closed_issues"`
	BlockedIssues           int     `json:"blocked_issues"`
	ReadyIssues             int     `json:"ready_issues"`
	EpicsEligibleForClosure int     `json:"epics_eligible_for_closure"`
	AverageLeadTimeHours    float64 `json:"average_lead_time_hours"`
}

// LabelCount from bd label list-all --json.
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// EpicStatus from bd epic status --json.
type EpicStatus struct {
	Epic             Issue `json:"epic"`
	TotalChildren    int   `json:"total_children"`
	ClosedChildren   int   `json:"closed_children"`
	EligibleForClose bool  `json:"eligible_for_close"`
}

// DaemonInfo from bd daemons list --json.
type DaemonInfo struct {
	WorkspacePath       string  `json:"WorkspacePath"`
	DatabasePath        string  `json:"DatabasePath"`
	SocketPath          string  `json:"SocketPath"`
	PID                 int     `json:"PID"`
	Version             string  `json:"Version"`
	UptimeSeconds       float64 `json:"UptimeSeconds"`
	LastActivityTime    string  `json:"LastActivityTime"`
	ExclusiveLockActive bool    `json:"ExclusiveLockActive"`
	ExclusiveLockHolder string  `json:"ExclusiveLockHolder"`
	Alive               bool    `json:"Alive"`
	Error               string  `json:"Error"`
}

// RepoData holds all polled data for a single beads repository.
type RepoData struct {
	Path     string // absolute path to the .beads directory
	Name     string // parent directory name (repo name)
	Stats    *Stats
	Issues   []Issue
	Blocked  []Issue
	Epics    []EpicStatus
	Labels   []LabelCount
	Error    error
	LastPoll time.Time
}

// BlockedIssue pairs a blocked issue with its originating repo.
type BlockedIssue struct {
	Issue    Issue
	RepoName string
}

// EpicWithRepo pairs an epic status with its originating repo.
type EpicWithRepo struct {
	EpicStatus
	RepoName string
}

// AggregateData holds data aggregated across every discovered repo.
type AggregateData struct {
	Repos           []*RepoData
	TotalIssues     int
	TotalOpen       int
	TotalInProgress int
	TotalBlocked    int
	TotalClosed     int
	TotalReady      int
	AvgLeadTime     float64
	AllLabels       []LabelCount
	PriorityDist    [5]int // index 0-4 = P0-P4
	AllBlocked      []BlockedIssue
	AllEpics        []EpicWithRepo
	Daemons         []DaemonInfo
	LastPoll        time.Time
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func priorityName(p int) string {
	if p >= 0 && p <= 4 {
		return fmt.Sprintf("P%d", p)
	}
	return fmt.Sprintf("P%d", p)
}

func priorityLabel(p int) string {
	switch p {
	case 0:
		return "Critical"
	case 1:
		return "High"
	case 2:
		return "Medium"
	case 3:
		return "Low"
	case 4:
		return "Backlog"
	default:
		return "Unknown"
	}
}

func priorityColor(p int) string {
	switch p {
	case 0:
		return "red"
	case 1:
		return "#ff8700"
	case 2:
		return "yellow"
	case 3:
		return "green"
	case 4:
		return "#888888"
	default:
		return "white"
	}
}

func statusColor(s string) string {
	switch s {
	case "open":
		return "green"
	case "in_progress":
		return "#5f87ff"
	case "blocked":
		return "red"
	case "closed":
		return "#888888"
	default:
		return "white"
	}
}

func statusLabel(s string) string {
	switch s {
	case "open":
		return "Open"
	case "in_progress":
		return "InProg"
	case "closed":
		return "Closed"
	case "blocked":
		return "Blocked"
	default:
		return s
	}
}

func typeLabel(t string) string {
	switch t {
	case "bug":
		return "bug"
	case "feature":
		return "feat"
	case "task":
		return "task"
	case "epic":
		return "epic"
	case "chore":
		return "chore"
	default:
		return t
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-2]) + ".."
}

func formatHours(hours float64) string {
	if hours <= 0 {
		return "n/a"
	}
	if hours < 1 {
		return fmt.Sprintf("%.0fm", hours*60)
	}
	if hours < 24 {
		return fmt.Sprintf("%.1fh", hours)
	}
	return fmt.Sprintf("%.1fd", hours/24)
}

func formatUptime(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.0fm", seconds/60)
	}
	h := int(seconds / 3600)
	m := int(seconds/60) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}

func parseShortDate(isoDate string) string {
	t, err := time.Parse(time.RFC3339, isoDate)
	if err != nil {
		// try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoDate)
		if err != nil {
			if len(isoDate) >= 10 {
				return isoDate[:10]
			}
			return isoDate
		}
	}
	return t.Format("2006-01-02")
}
