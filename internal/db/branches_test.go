package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBranchesAndTitles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	id, err := d.UpsertActiveFile(`/tmp/demo.txt`, time.Now())
	if err != nil {
		t.Fatalf("UpsertActiveFile: %v", err)
	}

	branches, err := d.ListBranches(id)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	if len(branches) != 1 || branches[0].Name != "main" {
		t.Fatalf("branches = %+v, want [main]", branches)
	}
	cur, err := d.CurrentBranch(id)
	if err != nil || cur != "main" {
		t.Fatalf("CurrentBranch = %q err=%v", cur, err)
	}

	hash1 := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	hash2 := "bbccddeeff00112233445566778899aabbccddeeff00112233445566778899aa"
	hash3 := "ccddeeff00112233445566778899aabbccddeeff00112233445566778899aabb"
	if _, err := d.InsertVersion(id, hash1, 1, time.Now(), nil, nil, "main"); err != nil {
		t.Fatalf("InsertVersion 1: %v", err)
	}
	if _, err := d.InsertVersion(id, hash2, 2, time.Now(), nil, nil, "main"); err != nil {
		t.Fatalf("InsertVersion 2: %v", err)
	}

	b, err := d.CreateBranch(id, "Exp/AI Agent", 2)
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if b.Name != "exp/ai-agent" || b.FromVersion != 2 {
		t.Fatalf("branch = %+v", b)
	}
	cur, err = d.CurrentBranch(id)
	if err != nil || cur != "exp/ai-agent" {
		t.Fatalf("CurrentBranch after create = %q err=%v", cur, err)
	}

	if _, err := d.InsertVersion(id, hash3, 3, time.Now(), nil, nil, cur); err != nil {
		t.Fatalf("InsertVersion 3: %v", err)
	}
	versions, err := d.ListVersions(id)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if versions[2].Branch != "exp/ai-agent" {
		t.Fatalf("v3 branch = %q", versions[2].Branch)
	}

	if err := d.SetVersionTitle(id, 3, "  explore agent hooks  "); err != nil {
		t.Fatalf("SetVersionTitle: %v", err)
	}
	v3, err := d.VersionByNumber(id, 3)
	if err != nil {
		t.Fatalf("VersionByNumber: %v", err)
	}
	if v3.Title != "explore agent hooks" {
		t.Fatalf("title = %q", v3.Title)
	}

	if err := d.SetCurrentBranch(id, "main"); err != nil {
		t.Fatalf("SetCurrentBranch: %v", err)
	}
	cur, _ = d.CurrentBranch(id)
	if cur != "main" {
		t.Fatalf("switched = %q", cur)
	}

	if _, err := d.CreateBranch(id, "exp/ai-agent", 2); err == nil {
		t.Fatal("expected duplicate branch error")
	}
}

func TestNormalizeBranchName(t *testing.T) {
	got, err := NormalizeBranchName("  Exp/AI Agent!! ")
	if err != nil {
		t.Fatalf("NormalizeBranchName: %v", err)
	}
	if got != "exp/ai-agent" {
		t.Fatalf("got %q", got)
	}
	if _, err := NormalizeBranchName("@@@"); err == nil {
		t.Fatal("expected error for empty cleaned name")
	}
}
