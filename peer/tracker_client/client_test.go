package tracker_client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestServer(mux *http.ServeMux) *httptest.Server {
	return httptest.NewServer(mux)
}

func TestUploadManifest_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		name := r.URL.Query().Get("name")
		if id == "" || name == "" {
			http.Error(w, "missing params", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	id := uuid.New()
	err := client.UploadManifest(id, "test.manifest", []byte("bencode data"))
	if err != nil {
		t.Fatalf("UploadManifest returned unexpected error: %v", err)
	}
}

func TestUploadManifest_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.UploadManifest(uuid.New(), "test", []byte("data"))
	if err == nil {
		t.Fatal("expected error for non-201 response, got nil")
	}
}

func TestUploadManifest_SendsCorrectQueryParams(t *testing.T) {
	id := uuid.New()
	wantName := "my-manifest"
	wantData := []byte("hello bencode")

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		gotID := r.URL.Query().Get("id")
		gotName := r.URL.Query().Get("name")
		if gotID != id.String() {
			t.Errorf("id mismatch: got %q, want %q", gotID, id.String())
		}
		if gotName != wantName {
			t.Errorf("name mismatch: got %q, want %q", gotName, wantName)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(wantData) {
			t.Errorf("body mismatch: got %q, want %q", body, wantData)
		}
		w.WriteHeader(http.StatusCreated)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.UploadManifest(id, wantName, wantData); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetManifest_Success(t *testing.T) {
	id := uuid.New()
	wantData := []byte("manifest binary content")

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(wantData)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetManifest(id)
	if err != nil {
		t.Fatalf("GetManifest returned unexpected error: %v", err)
	}
	if string(got) != string(wantData) {
		t.Errorf("data mismatch: got %q, want %q", got, wantData)
	}
}

func TestGetManifest_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.GetManifest(uuid.New())
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGetManifest_UsesCorrectURL(t *testing.T) {
	id := uuid.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest/", func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/manifest/" + id.String()
		if r.URL.Path != wantPath {
			t.Errorf("path mismatch: got %q, want %q", r.URL.Path, wantPath)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data"))
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetManifest(id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListManifests_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now().UTC()

	metas := []ManifestMeta{
		{ID: id1, Name: "first", CreatedAt: now},
		{ID: id2, Name: "second", CreatedAt: now},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/manifests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metas)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.ListManifests()
	if err != nil {
		t.Fatalf("ListManifests returned unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(got))
	}
	if got[0].ID != id1 && got[1].ID != id1 {
		t.Errorf("id1 %v not found in result", id1)
	}
}

func TestListManifests_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.ListManifests()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got))
	}
}

func TestListManifests_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifests", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.ListManifests()
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestRegisterPeer_Success(t *testing.T) {
	peerID := uuid.New()
	wantAddr := "127.0.0.1:9000"

	mux := http.NewServeMux()
	mux.HandleFunc("/peer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if body["id"] != peerID.String() {
			t.Errorf("id mismatch: got %q, want %q", body["id"], peerID.String())
		}
		if body["address"] != wantAddr {
			t.Errorf("address mismatch: got %q, want %q", body["address"], wantAddr)
		}
		w.WriteHeader(http.StatusCreated)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.RegisterPeer(peerID, wantAddr); err != nil {
		t.Fatalf("RegisterPeer returned unexpected error: %v", err)
	}
}

func TestRegisterPeer_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/peer", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.RegisterPeer(uuid.New(), "127.0.0.1:9000")
	if err == nil {
		t.Fatal("expected error for non-201 response, got nil")
	}
}

func TestAnnounce_Success(t *testing.T) {
	manifestID := uuid.New()
	peerID := uuid.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/announce", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if body["manifest_id"] != manifestID.String() {
			t.Errorf("manifest_id mismatch: got %q, want %q", body["manifest_id"], manifestID.String())
		}
		if body["peer_id"] != peerID.String() {
			t.Errorf("peer_id mismatch: got %q, want %q", body["peer_id"], peerID.String())
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.Announce(manifestID, peerID); err != nil {
		t.Fatalf("Announce returned unexpected error: %v", err)
	}
}

func TestAnnounce_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/announce", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusBadRequest)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.Announce(uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestGetSeeders_Success(t *testing.T) {
	manifestID := uuid.New()
	peerID := uuid.New()
	now := time.Now().UTC()

	peers := []PeerInfo{
		{ID: peerID, Address: "192.168.1.1:9001", LastSeen: now},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/peers/", func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/peers/" + manifestID.String()
		if r.URL.Path != wantPath {
			t.Errorf("path mismatch: got %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(peers)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetSeeders(manifestID)
	if err != nil {
		t.Fatalf("GetSeeders returned unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(got))
	}
	if got[0].ID != peerID {
		t.Errorf("peer ID mismatch: got %v, want %v", got[0].ID, peerID)
	}
	if got[0].Address != "192.168.1.1:9001" {
		t.Errorf("peer address mismatch: got %q, want %q", got[0].Address, "192.168.1.1:9001")
	}
}

func TestGetSeeders_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/peers/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.GetSeeders(uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got))
	}
}

func TestGetSeeders_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/peers/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	srv := newTestServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.GetSeeders(uuid.New())
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestNewClient_BaseURL(t *testing.T) {
	c := NewClient("http://localhost:8080")
	if c.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL mismatch: got %q, want %q", c.baseURL, "http://localhost:8080")
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}
