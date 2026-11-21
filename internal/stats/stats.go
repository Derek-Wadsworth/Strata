package stats

import (
	"path/filepath"
	"sort"
	"time"
)

// DefaultSessionGap is how long without a save before a session ends.
const DefaultSessionGap = 30 * time.Minute

// Event is one version snapshot used for aggregation.
type Event struct {
	Path         string
	Size         int64
	CreatedAt    time.Time
	LinesAdded   int
	LinesRemoved int
	HasLineStats bool
}

// DayCount is edits on a calendar day (UTC date key YYYY-MM-DD).
type DayCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// SizePoint is file size at a version timestamp.
type SizePoint struct {
	At   time.Time `json:"at"`
	Size int64     `json:"size"`
}

// FileStats summarizes one tracked file.
type FileStats struct {
	Path          string      `json:"path"`
	Name          string      `json:"name"`
	TotalVersions int         `json:"totalVersions"`
	FirstEdited   time.Time   `json:"firstEdited"`
	LastEdited    time.Time   `json:"lastEdited"`
	LinesAdded    int         `json:"linesAdded"`
	LinesRemoved  int         `json:"linesRemoved"`
	NetGrowth     int         `json:"netGrowth"`
	EditsPerDay   []DayCount  `json:"editsPerDay"`
	SizeOverTime  []SizePoint `json:"sizeOverTime"`
}

// FileActivity ranks files by version count.
type FileActivity struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Versions int    `json:"versions"`
}

// Session is a contiguous burst of saves.
type Session struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Saves   int       `json:"saves"`
	Minutes int       `json:"minutes"`
}

// Dashboard is the aggregate view for the GUI.
type Dashboard struct {
	TotalFiles             int            `json:"totalFiles"`
	TotalVersions          int            `json:"totalVersions"`
	LinesAdded             int            `json:"linesAdded"`
	LinesRemoved           int            `json:"linesRemoved"`
	EstimatedActiveMinutes int            `json:"estimatedActiveMinutes"`
	SessionCount           int            `json:"sessionCount"`
	Files                  []FileStats    `json:"files"`
	MostActive             []FileActivity `json:"mostActive"`
	EditsPerDay            []DayCount     `json:"editsPerDay"`
	Heatmap                []DayCount     `json:"heatmap"`
	Sessions               []Session      `json:"sessions"`
}

// BuildDashboard aggregates events into dashboard stats.
// sessionGap controls working-session detection; <=0 uses DefaultSessionGap.
func BuildDashboard(events []Event, sessionGap time.Duration) Dashboard {
	if sessionGap <= 0 {
		sessionGap = DefaultSessionGap
	}
	if len(events) == 0 {
		return Dashboard{
			Files:       []FileStats{},
			MostActive:  []FileActivity{},
			EditsPerDay: []DayCount{},
			Heatmap:     []DayCount{},
			Sessions:    []Session{},
		}
	}

	sorted := append([]Event(nil), events...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	byFile := map[string][]Event{}
	for _, e := range sorted {
		byFile[e.Path] = append(byFile[e.Path], e)
	}

	var (
		files     []FileStats
		most      []FileActivity
		allDays   = map[string]int{}
		totalAdd  int
		totalRem  int
		totalVers int
	)

	for path, fevents := range byFile {
		fs := fileStats(path, fevents)
		files = append(files, fs)
		most = append(most, FileActivity{
			Path:     path,
			Name:     filepath.Base(path),
			Versions: fs.TotalVersions,
		})
		totalVers += fs.TotalVersions
		totalAdd += fs.LinesAdded
		totalRem += fs.LinesRemoved
		for _, d := range fs.EditsPerDay {
			allDays[d.Day] += d.Count
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(most, func(i, j int) bool {
		if most[i].Versions == most[j].Versions {
			return most[i].Path < most[j].Path
		}
		return most[i].Versions > most[j].Versions
	})
	if len(most) > 10 {
		most = most[:10]
	}

	editsPerDay := dayCountsFromMap(allDays)
	sessions := buildSessions(sorted, sessionGap)
	activeMins := 0
	for _, s := range sessions {
		activeMins += s.Minutes
	}

	return Dashboard{
		TotalFiles:             len(files),
		TotalVersions:          totalVers,
		LinesAdded:             totalAdd,
		LinesRemoved:           totalRem,
		EstimatedActiveMinutes: activeMins,
		SessionCount:           len(sessions),
		Files:                  files,
		MostActive:             most,
		EditsPerDay:            editsPerDay,
		Heatmap:                editsPerDay,
		Sessions:               sessions,
	}
}

func fileStats(path string, events []Event) FileStats {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})
	days := map[string]int{}
	var added, removed int
	sizes := make([]SizePoint, 0, len(events))
	for _, e := range events {
		days[dayKey(e.CreatedAt)]++
		if e.HasLineStats {
			added += e.LinesAdded
			removed += e.LinesRemoved
		}
		sizes = append(sizes, SizePoint{At: e.CreatedAt, Size: e.Size})
	}
	return FileStats{
		Path:          path,
		Name:          filepath.Base(path),
		TotalVersions: len(events),
		FirstEdited:   events[0].CreatedAt,
		LastEdited:    events[len(events)-1].CreatedAt,
		LinesAdded:    added,
		LinesRemoved:  removed,
		NetGrowth:     added - removed,
		EditsPerDay:   dayCountsFromMap(days),
		SizeOverTime:  sizes,
	}
}

func buildSessions(events []Event, gap time.Duration) []Session {
	if len(events) == 0 {
		return nil
	}
	var out []Session
	start := events[0].CreatedAt
	end := events[0].CreatedAt
	saves := 1
	for i := 1; i < len(events); i++ {
		at := events[i].CreatedAt
		if at.Sub(end) > gap {
			out = append(out, sessionOf(start, end, saves))
			start = at
			end = at
			saves = 1
			continue
		}
		end = at
		saves++
	}
	out = append(out, sessionOf(start, end, saves))
	return out
}

func sessionOf(start, end time.Time, saves int) Session {
	mins := int(end.Sub(start).Minutes())
	if mins < 1 && saves > 0 {
		// A single save still counts as a short burst of work.
		mins = 1
	}
	return Session{Start: start, End: end, Saves: saves, Minutes: mins}
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func dayCountsFromMap(m map[string]int) []DayCount {
	out := make([]DayCount, 0, len(m))
	for day, count := range m {
		out = append(out, DayCount{Day: day, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}
