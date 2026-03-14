package http

import "testing"

func TestMapMobileCommandStatus(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "queued", want: "queued"},
		{in: "published", want: "sent"},
		{in: "sent", want: "sent"},
		{in: "acked", want: "acked"},
		{in: "completed", want: "acked"},
		{in: "timeout", want: "timed_out"},
		{in: "timed_out", want: "timed_out"},
		{in: "failed", want: "failed"},
		{in: "rejected", want: "failed"},
		{in: "unknown", want: "queued"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := mapMobileCommandStatus(tt.in)
			if got != tt.want {
				t.Fatalf("mapMobileCommandStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
