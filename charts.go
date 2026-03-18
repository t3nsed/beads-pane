package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(values []int, colorTag string) string {
	if len(values) == 0 {
		return ""
	}
	max := 0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	b.WriteString("[" + colorTag + "]")
	for _, v := range values {
		idx := 0
		if max > 0 {
			idx = v * 7 / max
		}
		if idx > 7 {
			idx = 7
		}
		b.WriteRune(sparkChars[idx])
	}
	b.WriteString("[-]")
	return b.String()
}

func sparklineF(values []float64, colorTag string) string {
	if len(values) == 0 {
		return ""
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	b.WriteString("[" + colorTag + "]")
	for _, v := range values {
		idx := 0
		if max > 0 {
			idx = int(v / max * 7)
		}
		if idx > 7 {
			idx = 7
		}
		b.WriteRune(sparkChars[idx])
	}
	b.WriteString("[-]")
	return b.String()
}

// computeBurndown returns daily open-issue counts for the last `days` days
// across all repos. Open at day N = (created on or before N) - (closed on or before N).
func computeBurndown(repos []*RepoData, days int) []int {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := startOfToday.AddDate(0, 0, -days+1)

	type event struct {
		date  time.Time
		delta int
	}
	var events []event

	for _, repo := range repos {
		for _, issue := range repo.Issues {
			created, err := time.Parse(time.RFC3339, issue.CreatedAt)
			if err != nil {
				continue
			}
			events = append(events, event{created, +1})

			if issue.ClosedAt != "" {
				closed, err := time.Parse(time.RFC3339, issue.ClosedAt)
				if err != nil {
					continue
				}
				events = append(events, event{closed, -1})
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].date.Before(events[j].date)
	})

	result := make([]int, days)
	eIdx := 0
	openCount := 0

	for eIdx < len(events) && events[eIdx].date.Before(start) {
		openCount += events[eIdx].delta
		eIdx++
	}

	for day := 0; day < days; day++ {
		dayEnd := start.AddDate(0, 0, day+1)
		for eIdx < len(events) && events[eIdx].date.Before(dayEnd) {
			openCount += events[eIdx].delta
			eIdx++
		}
		if openCount < 0 {
			openCount = 0
		}
		result[day] = openCount
	}

	return result
}

// computeThroughput returns the number of issues closed in each of the last `weeks` weeks.
func computeThroughput(repos []*RepoData, weeks int) []int {
	now := time.Now()
	result := make([]int, weeks)

	for _, repo := range repos {
		for _, issue := range repo.Issues {
			if issue.ClosedAt == "" {
				continue
			}
			closed, err := time.Parse(time.RFC3339, issue.ClosedAt)
			if err != nil {
				continue
			}
			hoursAgo := now.Sub(closed).Hours()
			if hoursAgo < 0 {
				continue
			}
			weekIdx := weeks - 1 - int(hoursAgo/(24*7))
			if weekIdx >= 0 && weekIdx < weeks {
				result[weekIdx]++
			}
		}
	}
	return result
}

// computeLeadTime returns average lead-time in hours for issues closed in each
// of the last `weeks` weeks.
func computeLeadTime(repos []*RepoData, weeks int) []float64 {
	now := time.Now()
	sums := make([]float64, weeks)
	counts := make([]int, weeks)

	for _, repo := range repos {
		for _, issue := range repo.Issues {
			if issue.ClosedAt == "" {
				continue
			}
			created, err := time.Parse(time.RFC3339, issue.CreatedAt)
			if err != nil {
				continue
			}
			closed, err := time.Parse(time.RFC3339, issue.ClosedAt)
			if err != nil {
				continue
			}
			leadHours := closed.Sub(created).Hours()
			if leadHours < 0 {
				continue
			}
			hoursAgo := now.Sub(closed).Hours()
			if hoursAgo < 0 {
				continue
			}
			weekIdx := weeks - 1 - int(hoursAgo/(24*7))
			if weekIdx >= 0 && weekIdx < weeks {
				sums[weekIdx] += leadHours
				counts[weekIdx]++
			}
		}
	}

	result := make([]float64, weeks)
	for i := range result {
		if counts[i] > 0 {
			result[i] = sums[i] / float64(counts[i])
		}
	}
	return result
}

func renderCharts(repos []*RepoData, theme *Theme) string {
	burndown := computeBurndown(repos, 30)
	throughput := computeThroughput(repos, 12)
	leadTime := computeLeadTime(repos, 12)

	var b strings.Builder

	first, last := 0, 0
	if len(burndown) > 0 {
		first = burndown[0]
		last = burndown[len(burndown)-1]
	}
	fmt.Fprintf(&b, " [%s]Burndown 30d[-]  %s  %d%s%d\n",
		theme.DimTag, sparkline(burndown, theme.AccentTag), first, "→", last)

	throughputAvg := 0.0
	tpTotal := 0
	for _, v := range throughput {
		tpTotal += v
	}
	if len(throughput) > 0 {
		throughputAvg = float64(tpTotal) / float64(len(throughput))
	}
	fmt.Fprintf(&b, " [%s]Closed/wk[-]    %s  avg %.1f\n",
		theme.DimTag, sparkline(throughput, theme.ReadyTag), throughputAvg)

	ltAvg := 0.0
	ltCount := 0
	for _, v := range leadTime {
		if v > 0 {
			ltAvg += v
			ltCount++
		}
	}
	if ltCount > 0 {
		ltAvg /= float64(ltCount)
	}
	fmt.Fprintf(&b, " [%s]Lead time[-]    %s  avg %s",
		theme.DimTag, sparklineF(leadTime, theme.InProgTag), formatHours(ltAvg))

	return b.String()
}
