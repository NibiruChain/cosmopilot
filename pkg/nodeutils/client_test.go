package nodeutils

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("localhost")
	expected := "http://localhost:8000"
	if client.url != expected {
		t.Errorf("expected url %s, got %s", expected, client.url)
	}
}

func TestClient_GetDataSize(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		want       int64
		wantErr    bool
	}{
		{
			name:       "valid response",
			response:   "1024",
			statusCode: http.StatusOK,
			want:       1024,
			wantErr:    false,
		},
		{
			name:       "large size",
			response:   "1099511627776",
			statusCode: http.StatusOK,
			want:       1099511627776,
			wantErr:    false,
		},
		{
			name:       "server error",
			response:   "internal error",
			statusCode: http.StatusInternalServerError,
			want:       0,
			wantErr:    true,
		},
		{
			name:       "invalid response",
			response:   "not a number",
			statusCode: http.StatusOK,
			want:       0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/data_size" {
					t.Errorf("expected path /data_size, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.GetDataSize(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDataSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetDataSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetLatestHeight(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		want       int64
		wantErr    bool
	}{
		{
			name:       "valid response",
			response:   "12345",
			statusCode: http.StatusOK,
			want:       12345,
			wantErr:    false,
		},
		{
			name:       "zero height",
			response:   "0",
			statusCode: http.StatusOK,
			want:       0,
			wantErr:    false,
		},
		{
			name:       "server error",
			response:   "error",
			statusCode: http.StatusInternalServerError,
			want:       0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/latest_height" {
					t.Errorf("expected path /latest_height, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.GetLatestHeight(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestHeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLatestHeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_RequiresUpgrade(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		want       bool
		wantErr    bool
	}{
		{
			name:       "no upgrade required",
			response:   "false",
			statusCode: http.StatusOK,
			want:       false,
			wantErr:    false,
		},
		{
			name:       "upgrade required with 200",
			response:   "true",
			statusCode: http.StatusOK,
			want:       true,
			wantErr:    false,
		},
		{
			name:       "upgrade required with 426",
			response:   "true",
			statusCode: http.StatusUpgradeRequired,
			want:       true,
			wantErr:    false,
		},
		{
			name:       "server error",
			response:   "error",
			statusCode: http.StatusInternalServerError,
			want:       false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/must_upgrade" {
					t.Errorf("expected path /must_upgrade, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.RequiresUpgrade(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("RequiresUpgrade() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("RequiresUpgrade() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_ListSnapshots(t *testing.T) {
	tests := []struct {
		name       string
		response   []int64
		statusCode int
		want       []int64
		wantErr    bool
	}{
		{
			name:       "multiple snapshots",
			response:   []int64{1000, 2000, 3000},
			statusCode: http.StatusOK,
			want:       []int64{1000, 2000, 3000},
			wantErr:    false,
		},
		{
			name:       "empty list",
			response:   []int64{},
			statusCode: http.StatusOK,
			want:       []int64{},
			wantErr:    false,
		},
		{
			name:       "single snapshot",
			response:   []int64{5000},
			statusCode: http.StatusOK,
			want:       []int64{5000},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/snapshots" {
					t.Errorf("expected path /snapshots, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.ListSnapshots(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListSnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ListSnapshots() returned %d items, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ListSnapshots()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestClient_GetCPUStats(t *testing.T) {
	tests := []struct {
		name       string
		since      time.Duration
		response   string
		statusCode int
		want       float64
		wantErr    bool
		checkQuery bool
		wantQuery  string
	}{
		{
			name:       "live stats",
			since:      0,
			response:   "45.5",
			statusCode: http.StatusOK,
			want:       45.5,
			wantErr:    false,
			checkQuery: false,
		},
		{
			name:       "average over 1h",
			since:      time.Hour,
			response:   "30.2",
			statusCode: http.StatusOK,
			want:       30.2,
			wantErr:    false,
			checkQuery: true,
			wantQuery:  "1h0m0s",
		},
		{
			name:       "server error",
			since:      0,
			response:   "error",
			statusCode: http.StatusInternalServerError,
			want:       0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stats/cpu" {
					t.Errorf("expected path /stats/cpu, got %s", r.URL.Path)
				}
				if tt.checkQuery {
					avg := r.URL.Query().Get("average")
					if avg != tt.wantQuery {
						t.Errorf("expected query average=%s, got %s", tt.wantQuery, avg)
					}
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.GetCPUStats(context.Background(), tt.since)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCPUStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetCPUStats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetMemoryStats(t *testing.T) {
	tests := []struct {
		name       string
		since      time.Duration
		response   string
		statusCode int
		want       uint64
		wantErr    bool
	}{
		{
			name:       "live stats",
			since:      0,
			response:   "1073741824",
			statusCode: http.StatusOK,
			want:       1073741824,
			wantErr:    false,
		},
		{
			name:       "average over 1h",
			since:      time.Hour,
			response:   "2147483648",
			statusCode: http.StatusOK,
			want:       2147483648,
			wantErr:    false,
		},
		{
			name:       "server error",
			since:      0,
			response:   "error",
			statusCode: http.StatusInternalServerError,
			want:       0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stats/memory" {
					t.Errorf("expected path /stats/memory, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.GetMemoryStats(context.Background(), tt.since)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMemoryStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetMemoryStats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_IsStateSyncing(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		want       bool
		wantErr    bool
	}{
		{
			name:       "not syncing",
			response:   "false",
			statusCode: http.StatusOK,
			want:       false,
			wantErr:    false,
		},
		{
			name:       "syncing",
			response:   "true",
			statusCode: http.StatusOK,
			want:       true,
			wantErr:    false,
		},
		{
			name:       "server error",
			response:   "error",
			statusCode: http.StatusInternalServerError,
			want:       false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/state_syncing" {
					t.Errorf("expected path /state_syncing, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			got, err := client.IsStateSyncing(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("IsStateSyncing() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsStateSyncing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_ShutdownNodeUtilsServer(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful shutdown",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    false, // ShutdownNodeUtilsServer only checks if request was sent
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/shutdown" {
					t.Errorf("expected path /shutdown, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected method POST, got %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := &Client{url: server.URL}
			err := client.ShutdownNodeUtilsServer(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ShutdownNodeUtilsServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("100"))
	}))
	defer server.Close()

	client := &Client{url: server.URL}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetLatestHeight(ctx)
	if err == nil {
		t.Error("expected error due to cancelled context, got nil")
	}
}
