package store

import (
	"testing"

	"github.com/google/uuid"
)

func TestSaveAndGetManifest(t *testing.T) {
	s := NewInMemoryStore()
	id := uuid.New()
	data := []byte("bencode manifest data")

	if err := s.SaveManifest(id, "test.manifest", data); err != nil {
		t.Fatalf("SaveManifest error: %v", err)
	}

	got, err := s.GetManifest(id)
	if err != nil {
		t.Fatalf("GetManifest error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", got, data)
	}
}

func TestGetManifestNotFound(t *testing.T) {
	s := NewInMemoryStore()
	_, err := s.GetManifest(uuid.New())
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
}

func TestSaveManifestIsolatesData(t *testing.T) {
	s := NewInMemoryStore()
	id := uuid.New()
	data := []byte("original")

	s.SaveManifest(id, "f", data)

	data[0] = 'X'

	got, _ := s.GetManifest(id)
	if got[0] == 'X' {
		t.Error("store holds reference to original slice, expected a copy")
	}
}

func TestListManifests(t *testing.T) {
	s := NewInMemoryStore()

	id1, id2 := uuid.New(), uuid.New()
	s.SaveManifest(id1, "first", []byte("a"))
	s.SaveManifest(id2, "second", []byte("b"))

	list, err := s.ListManifests()
	if err != nil {
		t.Fatalf("ListManifests error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(list))
	}
}

func TestListManifestsEmpty(t *testing.T) {
	s := NewInMemoryStore()
	list, err := s.ListManifests()
	if err != nil {
		t.Fatalf("ListManifests error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestRegisterAndGetPeer(t *testing.T) {
	s := NewInMemoryStore()
	peerID := uuid.New()

	if err := s.RegisterPeer(peerID, "127.0.0.1:9000"); err != nil {
		t.Fatalf("RegisterPeer error: %v", err)
	}

	peer, err := s.GetPeer(peerID)
	if err != nil {
		t.Fatalf("GetPeer error: %v", err)
	}
	if peer.ID != peerID {
		t.Errorf("ID: got %v, want %v", peer.ID, peerID)
	}
	if peer.Address != "127.0.0.1:9000" {
		t.Errorf("Address: got %q, want %q", peer.Address, "127.0.0.1:9000")
	}
}

func TestGetPeerNotFound(t *testing.T) {
	s := NewInMemoryStore()
	_, err := s.GetPeer(uuid.New())
	if err == nil {
		t.Fatal("expected error for missing peer, got nil")
	}
}

func TestRegisterPeerUpdatesAddress(t *testing.T) {
	s := NewInMemoryStore()
	peerID := uuid.New()

	s.RegisterPeer(peerID, "127.0.0.1:9000")
	s.RegisterPeer(peerID, "127.0.0.1:9001")

	peer, _ := s.GetPeer(peerID)
	if peer.Address != "127.0.0.1:9001" {
		t.Errorf("expected updated address, got %q", peer.Address)
	}
}

func TestAnnounceAndGetSeeders(t *testing.T) {
	s := NewInMemoryStore()
	manifestID := uuid.New()
	peerID := uuid.New()

	s.SaveManifest(manifestID, "m", []byte("data"))
	s.RegisterPeer(peerID, "127.0.0.1:9000")

	if err := s.AnnounceSeeder(manifestID, peerID); err != nil {
		t.Fatalf("AnnounceSeeder error: %v", err)
	}

	seeders, err := s.GetSeeders(manifestID)
	if err != nil {
		t.Fatalf("GetSeeders error: %v", err)
	}
	if len(seeders) != 1 {
		t.Fatalf("expected 1 seeder, got %d", len(seeders))
	}
	if seeders[0].ID != peerID {
		t.Errorf("seeder ID: got %v, want %v", seeders[0].ID, peerID)
	}
}

func TestGetSeedersEmptyForUnknownManifest(t *testing.T) {
	s := NewInMemoryStore()
	seeders, err := s.GetSeeders(uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seeders) != 0 {
		t.Errorf("expected empty list, got %d", len(seeders))
	}
}

func TestAnnounceSeederManifestNotFound(t *testing.T) {
	s := NewInMemoryStore()
	peerID := uuid.New()
	s.RegisterPeer(peerID, "127.0.0.1:9000")

	err := s.AnnounceSeeder(uuid.New(), peerID)
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
}

func TestAnnounceSeederPeerNotFound(t *testing.T) {
	s := NewInMemoryStore()
	manifestID := uuid.New()
	s.SaveManifest(manifestID, "m", []byte("data"))

	err := s.AnnounceSeeder(manifestID, uuid.New())
	if err == nil {
		t.Fatal("expected error for missing peer, got nil")
	}
}

func TestMultipleSeedersForManifest(t *testing.T) {
	s := NewInMemoryStore()
	manifestID := uuid.New()
	s.SaveManifest(manifestID, "m", []byte("data"))

	peer1, peer2 := uuid.New(), uuid.New()
	s.RegisterPeer(peer1, "127.0.0.1:9001")
	s.RegisterPeer(peer2, "127.0.0.1:9002")

	s.AnnounceSeeder(manifestID, peer1)
	s.AnnounceSeeder(manifestID, peer2)

	seeders, _ := s.GetSeeders(manifestID)
	if len(seeders) != 2 {
		t.Errorf("expected 2 seeders, got %d", len(seeders))
	}
}

func TestAnnounceSeederIdempotent(t *testing.T) {
	s := NewInMemoryStore()
	manifestID := uuid.New()
	peerID := uuid.New()

	s.SaveManifest(manifestID, "m", []byte("data"))
	s.RegisterPeer(peerID, "127.0.0.1:9000")

	s.AnnounceSeeder(manifestID, peerID)
	s.AnnounceSeeder(manifestID, peerID)

	seeders, _ := s.GetSeeders(manifestID)
	if len(seeders) != 1 {
		t.Errorf("expected 1 seeder (idempotent), got %d", len(seeders))
	}
}
