package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// readOnlyFlags passed to every bd invocation to guarantee no writes.
var readOnlyFlags = []string{"--sandbox", "--no-auto-flush", "--no-auto-import"}

// runBD executes bd targeting a specific .beads directory.
// All invocations are read-only thanks to readOnlyFlags.
func runBD(beadsDir string, args ...string) ([]byte, error) {
	allArgs := make([]string, 0, len(args)+len(readOnlyFlags))
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, readOnlyFlags...)

	cmd := exec.Command("bd", allArgs...)
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// runBDGlobal executes bd without targeting a specific repo.
func runBDGlobal(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// pollRepo gathers all data for a single beads repository.
func pollRepo(beadsDir string) *RepoData {
	repoPath := filepath.Dir(beadsDir)
	repoName := filepath.Base(repoPath)

	rd := &RepoData{
		Path:     beadsDir,
		Name:     repoName,
		LastPoll: time.Now(),
	}

	// Stats
	if out, err := runBD(beadsDir, "stats", "--json"); err == nil {
		var s Stats
		if json.Unmarshal(out, &s) == nil {
			rd.Stats = &s
		}
	}

	// All issues
	if out, err := runBD(beadsDir, "list", "--json"); err == nil {
		var issues []Issue
		if json.Unmarshal(out, &issues) == nil {
			rd.Issues = issues
		}
	}

	// Blocked (includes blocked_by detail)
	if out, err := runBD(beadsDir, "blocked", "--json"); err == nil {
		var blocked []Issue
		if json.Unmarshal(out, &blocked) == nil {
			rd.Blocked = blocked
		}
	}

	// Epics
	if out, err := runBD(beadsDir, "epic", "status", "--json"); err == nil {
		trimmed := bytes.TrimSpace(out)
		if len(trimmed) > 0 && string(trimmed) != "null" {
			var epics []EpicStatus
			if json.Unmarshal(trimmed, &epics) == nil {
				rd.Epics = epics
			}
		}
	}

	// Labels
	if out, err := runBD(beadsDir, "label", "list-all", "--json"); err == nil {
		var labels []LabelCount
		if json.Unmarshal(out, &labels) == nil {
			rd.Labels = labels
		}
	}

	return rd
}

// pollAllRepos polls every repo in parallel and returns aggregated data.
func pollAllRepos(beadsDirs []string) *AggregateData {
	repos := make([]*RepoData, len(beadsDirs))

	var wg sync.WaitGroup
	for i, dir := range beadsDirs {
		wg.Add(1)
		go func(idx int, bd string) {
			defer wg.Done()
			repos[idx] = pollRepo(bd)
		}(i, dir)
	}
	wg.Wait()

	// Daemon status is global, not per-repo.
	var daemons []DaemonInfo
	if out, err := runBDGlobal("daemons", "list", "--json"); err == nil {
		_ = json.Unmarshal(out, &daemons)
	}

	return aggregate(repos, daemons)
}

// aggregate computes cross-repo totals.
func aggregate(repos []*RepoData, daemons []DaemonInfo) *AggregateData {
	agg := &AggregateData{
		Repos:    repos,
		Daemons:  daemons,
		LastPoll: time.Now(),
	}

	labelMap := make(map[string]int)
	var leadTimeSum float64
	var leadTimeCount int

	for _, repo := range repos {
		if repo.Stats != nil {
			agg.TotalIssues += repo.Stats.TotalIssues
			agg.TotalOpen += repo.Stats.OpenIssues
			agg.TotalInProgress += repo.Stats.InProgressIssues
			agg.TotalBlocked += repo.Stats.BlockedIssues
			agg.TotalClosed += repo.Stats.ClosedIssues
			agg.TotalReady += repo.Stats.ReadyIssues

			if repo.Stats.AverageLeadTimeHours > 0 {
				leadTimeSum += repo.Stats.AverageLeadTimeHours
				leadTimeCount++
			}
		}

		// Priority distribution across all active issues.
		for _, issue := range repo.Issues {
			if issue.Status != "closed" && issue.Priority >= 0 && issue.Priority <= 4 {
				agg.PriorityDist[issue.Priority]++
			}
		}

		// Blocked issues tagged with repo name.
		for _, b := range repo.Blocked {
			agg.AllBlocked = append(agg.AllBlocked, BlockedIssue{
				Issue:    b,
				RepoName: repo.Name,
			})
		}

		// Epics tagged with repo name.
		for _, e := range repo.Epics {
			agg.AllEpics = append(agg.AllEpics, EpicWithRepo{
				EpicStatus: e,
				RepoName:   repo.Name,
			})
		}

		// Merge labels.
		for _, lc := range repo.Labels {
			labelMap[lc.Label] += lc.Count
		}
	}

	if leadTimeCount > 0 {
		agg.AvgLeadTime = leadTimeSum / float64(leadTimeCount)
	}

	// Sorted label list (descending by count).
	for label, count := range labelMap {
		agg.AllLabels = append(agg.AllLabels, LabelCount{Label: label, Count: count})
	}
	sort.Slice(agg.AllLabels, func(i, j int) bool {
		return agg.AllLabels[i].Count > agg.AllLabels[j].Count
	})

	// Sort repos by total issue count descending.
	sort.Slice(agg.Repos, func(i, j int) bool {
		ci, cj := 0, 0
		if agg.Repos[i].Stats != nil {
			ci = agg.Repos[i].Stats.TotalIssues
		}
		if agg.Repos[j].Stats != nil {
			cj = agg.Repos[j].Stats.TotalIssues
		}
		return ci > cj
	})

	return agg
}
