package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tracker/embedder"
	"tracker/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(s store.TrackerStore) *gin.Engine {
	srv := NewServer(s, &embedder.NoopEmbedder{})
	r := gin.New()
	r.GET("/hello", srv.hello)
	r.POST("/manifest", srv.uploadManifest)
	r.GET("/manifest/:id", srv.getManifest)
	r.GET("/manifests", srv.listManifests)
	r.POST("/peer", srv.registerPeer)
	r.POST("/announce", srv.announce)
	r.GET("/peers/:manifestID", srv.getSeeders)
	r.POST("/search", srv.search)
	return r
}

func TestHello(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestUploadManifest(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	id := uuid.New()
	body := []byte("bencode manifest bytes")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/manifest?id="+id.String()+"&name=test.manifest&description=some+manifest+description", bytes.NewReader(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadManifestMissingParams(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/manifest?name=test&description=d", bytes.NewReader([]byte("data")))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/manifest?id="+uuid.New().String()+"&description=d", bytes.NewReader([]byte("data")))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/manifest?id="+uuid.New().String()+"&name=test", bytes.NewReader([]byte("data")))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing description, got %d", w.Code)
	}
}

func TestUploadManifestInvalidID(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/manifest?id=not-a-uuid&name=test&description=d", bytes.NewReader([]byte("data")))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetManifest(t *testing.T) {
	s := store.NewInMemoryStore()
	r := setupRouter(s)

	id := uuid.New()
	data := []byte("manifest content")
	s.SaveManifest(id, "test", "desc", nil, data)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/manifest/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != string(data) {
		t.Errorf("body mismatch: got %q, want %q", w.Body.String(), data)
	}
}

func TestGetManifestNotFound(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/manifest/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetManifestInvalidID(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/manifest/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListManifests(t *testing.T) {
	s := store.NewInMemoryStore()
	r := setupRouter(s)

	s.SaveManifest(uuid.New(), "first", "desc1", nil, []byte("a"))
	s.SaveManifest(uuid.New(), "second", "desc2", nil, []byte("b"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/manifests", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 manifests, got %d", len(result))
	}
}

func TestRegisterPeer(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	peerID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"id":      peerID.String(),
		"address": "127.0.0.1:9000",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/peer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterPeerMissingFields(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())

	body, _ := json.Marshal(map[string]string{"id": uuid.New().String()})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/peer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing address, got %d", w.Code)
	}
}

func TestAnnounce(t *testing.T) {
	s := store.NewInMemoryStore()
	r := setupRouter(s)

	manifestID := uuid.New()
	peerID := uuid.New()
	s.SaveManifest(manifestID, "m", "desc", nil, []byte("data"))
	s.RegisterPeer(peerID, "127.0.0.1:9000")

	body, _ := json.Marshal(map[string]string{
		"manifest_id": manifestID.String(),
		"peer_id":     peerID.String(),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/announce", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnnounceManifestNotFound(t *testing.T) {
	s := store.NewInMemoryStore()
	r := setupRouter(s)

	peerID := uuid.New()
	s.RegisterPeer(peerID, "127.0.0.1:9000")

	body, _ := json.Marshal(map[string]string{
		"manifest_id": uuid.New().String(),
		"peer_id":     peerID.String(),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/announce", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSeeders(t *testing.T) {
	s := store.NewInMemoryStore()
	r := setupRouter(s)

	manifestID := uuid.New()
	peerID := uuid.New()
	s.SaveManifest(manifestID, "m", "desc", nil, []byte("data"))
	s.RegisterPeer(peerID, "127.0.0.1:9000")
	s.AnnounceSeeder(manifestID, peerID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/peers/"+manifestID.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var peers []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &peers); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(peers))
	}
}

func TestGetSeedersInvalidID(t *testing.T) {
	r := setupRouter(store.NewInMemoryStore())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/peers/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
