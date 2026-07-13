package taskx

import (
	"errors"
	"testing"
	"time"
)

func TestManagedJobValidate(t *testing.T) {
	tests := []struct {
		name    string
		job     ManagedJob
		wantErr bool
	}{
		{name: "interval", job: ManagedJob{Name: "grant-expirer", Schedule: "@every 5m"}},
		{name: "one shot", job: ManagedJob{Name: "grant-expirer", DueTime: "30s"}},
		{name: "missing name", job: ManagedJob{Schedule: "@every 5m"}, wantErr: true},
		{name: "missing schedule", job: ManagedJob{Name: "grant-expirer"}, wantErr: true},
		{
			name: "invalid drop policy",
			job: ManagedJob{
				Name:     "grant-expirer",
				Schedule: "@every 5m",
				FailurePolicy: &DeliveryFailurePolicy{
					Mode:     DeliveryFailureDrop,
					Interval: time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if tt.wantErr && !errors.Is(err, ErrInvalidJob) {
				t.Fatalf("Validate() error = %v, want ErrInvalidJob", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
