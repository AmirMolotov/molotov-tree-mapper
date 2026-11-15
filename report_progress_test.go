package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReportProgressAndCancellation(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string][]byte{"a.txt": []byte("hello"), "b.bin": {0, 1}} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	var final ReportProgress
	text, err := reportWithProgress(context.Background(), root, tree, nil, 0, func(p ReportProgress) { final = p })
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != "Complete" || final.Processed != 2 || final.Total != 2 || final.Included != 1 || final.Skipped != 1 || final.SourceBytes != 5 || final.OutputBytes != len(text) {
		t.Fatalf("wrong progress: %+v", final)
	}
	ctx, cancel := context.WithCancel(context.Background())
	text, err = reportWithProgress(ctx, root, tree, nil, 0, func(p ReportProgress) {
		if p.Processed == 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || text != "" {
		t.Fatalf("cancel returned %q, %v", text, err)
	}
	if _, err = scanSelectedContext(ctx, root, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan ignored cancellation: %v", err)
	}
	if _, err = safeReadContext(ctx, root, "a.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("read ignored cancellation: %v", err)
	}
}

func TestCancelReportWithoutAppLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{cancelReport: cancel}
	a.mu.Lock()
	a.CancelReport()
	a.mu.Unlock()
	if ctx.Err() == nil {
		t.Fatal("cancel did not reach active report")
	}
}

func TestReportBytesAdvanceBeforeFileCompletes(t *testing.T) {
	root := t.TempDir()
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), data, 0600); err != nil {
		t.Fatal(err)
	}
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	previous := 0
	text, err := reportWithProgress(context.Background(), root, tree, nil, 0, func(p ReportProgress) {
		if p.ReadBytes < previous {
			t.Fatal("read count moved backwards")
		}
		if p.ReadBytes > previous && p.Processed == 0 {
			updates++
		}
		previous = p.ReadBytes
	})
	if err != nil {
		t.Fatal(err)
	}
	if updates < 2 || previous != len(data) || len(text) <= previous {
		t.Fatalf("missing live byte progress: updates=%d read=%d", updates, previous)
	}
}
