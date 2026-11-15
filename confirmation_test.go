package main

import (
	"context"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"testing"
)

func TestHistoryConfirmation(t *testing.T) {
	for _, answer := range []string{"Yes", "No", "", "Cancel"} {
		t.Run(answer, func(t *testing.T) {
			confirmed, err := confirmHistoryChange(context.Background(), "Revert version", "Revert managed files?", func(_ context.Context, opts runtime.MessageDialogOptions) (string, error) {
				if opts.Type != runtime.QuestionDialog || opts.DefaultButton != "No" || opts.CancelButton != "No" || len(opts.Buttons) != 2 || opts.Buttons[0] != "Yes" || opts.Buttons[1] != "No" {
					t.Fatalf("unsafe confirmation options: %+v", opts)
				}
				return answer, nil
			})
			if err != nil || confirmed != (answer == "Yes") {
				t.Fatalf("answer %q: confirmed=%v, err=%v", answer, confirmed, err)
			}
		})
	}
	failure := errors.New("dialog failed")
	confirmed, err := confirmHistoryChange(context.Background(), "Revert", "Proceed?", func(context.Context, runtime.MessageDialogOptions) (string, error) { return "Yes", failure })
	if confirmed || !errors.Is(err, failure) {
		t.Fatalf("dialog error accepted: %v, %v", confirmed, err)
	}
}
