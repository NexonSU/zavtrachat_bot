package main

import (
	"errors"
	"testing"
	"time"
)

func TestStopWithin(t *testing.T) {
	errUpdater := errors.New("updater")
	errReminders := errors.New("reminders")
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	tests := []struct {
		name    string
		stops   []func() error
		wantErr []error
	}{
		{
			name:  "all stop in time",
			stops: []func() error{func() error { return nil }, func() error { return nil }},
		},
		{
			name:    "errors are joined and every stop runs",
			stops:   []func() error{func() error { return errUpdater }, func() error { return errReminders }},
			wantErr: []error{errUpdater, errReminders},
		},
		{
			name:    "hanging stop hits the timeout",
			stops:   []func() error{func() error { <-release; return nil }},
			wantErr: []error{errShutdownTimeout},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			err := stopWithin(50*time.Millisecond, tt.stops...)
			if len(tt.wantErr) == 0 && err != nil {
				t.Fatalf("stopWithin: err = %v, want nil", err)
			}
			for _, want := range tt.wantErr {
				if !errors.Is(err, want) {
					t.Errorf("stopWithin: err = %v, want it to wrap %v", err, want)
				}
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("stopWithin took %v, want it bounded by the timeout", elapsed)
			}
		})
	}
}
