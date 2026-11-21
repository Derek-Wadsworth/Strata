package stats

import (
	"testing"
	"time"
)

func TestBuildDashboardKnownDataset(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	events := []Event{
		{Path: `/a.txt`, Size: 10, CreatedAt: day.Add(10 * time.Hour), LinesAdded: 0, LinesRemoved: 0, HasLineStats: false},
		{Path: `/a.txt`, Size: 20, CreatedAt: day.Add(10*time.Hour + 5*time.Minute), LinesAdded: 5, LinesRemoved: 1, HasLineStats: true},
		{Path: `/a.txt`, Size: 25, CreatedAt: day.Add(12 * time.Hour), LinesAdded: 2, LinesRemoved: 0, HasLineStats: true},
		{Path: `/b.txt`, Size: 3, CreatedAt: day.Add(10*time.Hour + 2*time.Minute), LinesAdded: 0, LinesRemoved: 0, HasLineStats: false},
		{Path: `/b.txt`, Size: 8, CreatedAt: day.Add(48 * time.Hour), LinesAdded: 4, LinesRemoved: 1, HasLineStats: true},
	}

	dash := BuildDashboard(events, DefaultSessionGap)

	if dash.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d", dash.TotalFiles)
	}
	if dash.TotalVersions != 5 {
		t.Fatalf("TotalVersions = %d", dash.TotalVersions)
	}
	// Line stats only count versions that have them: a +5-1 +2-0, b +4-1 => +11 -2
	if dash.LinesAdded != 11 || dash.LinesRemoved != 2 {
		t.Fatalf("lines = +%d -%d", dash.LinesAdded, dash.LinesRemoved)
	}
	if len(dash.MostActive) < 1 || dash.MostActive[0].Path != `/a.txt` || dash.MostActive[0].Versions != 3 {
		t.Fatalf("MostActive = %+v", dash.MostActive)
	}

	// Days: 2026-11-20 has 4 events, 2026-11-22 has 1
	if len(dash.EditsPerDay) != 2 {
		t.Fatalf("EditsPerDay = %+v", dash.EditsPerDay)
	}
	if dash.EditsPerDay[0].Day != "2026-11-20" || dash.EditsPerDay[0].Count != 4 {
		t.Fatalf("day0 = %+v", dash.EditsPerDay[0])
	}
	if dash.EditsPerDay[1].Day != "2026-11-22" || dash.EditsPerDay[1].Count != 1 {
		t.Fatalf("day1 = %+v", dash.EditsPerDay[1])
	}

	// Sessions (30m gap): 10:00–10:05 (3 saves), 12:00 alone, then +48h alone => 3
	if dash.SessionCount != 3 {
		t.Fatalf("SessionCount = %d, sessions=%+v", dash.SessionCount, dash.Sessions)
	}
	if dash.Sessions[0].Saves != 3 {
		t.Fatalf("session0 saves = %d", dash.Sessions[0].Saves)
	}
	if dash.EstimatedActiveMinutes != 7 {
		t.Fatalf("EstimatedActiveMinutes = %d", dash.EstimatedActiveMinutes)
	}

	var a FileStats
	for _, f := range dash.Files {
		if f.Path == `/a.txt` {
			a = f
		}
	}
	if a.TotalVersions != 3 || a.NetGrowth != 6 {
		t.Fatalf("file a stats = %+v", a)
	}
	if len(a.SizeOverTime) != 3 || a.SizeOverTime[2].Size != 25 {
		t.Fatalf("size over time = %+v", a.SizeOverTime)
	}
}

func TestBuildSessionsGap(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Path: "x", CreatedAt: t0},
		{Path: "x", CreatedAt: t0.Add(10 * time.Minute)},
		{Path: "x", CreatedAt: t0.Add(50 * time.Minute)},
	}
	sessions := buildSessions(events, 30*time.Minute)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %+v", sessions)
	}
	if sessions[0].Minutes != 10 {
		t.Fatalf("session0 minutes = %d", sessions[0].Minutes)
	}
	if sessions[1].Minutes != 1 {
		t.Fatalf("single-save session minutes = %d", sessions[1].Minutes)
	}
}
