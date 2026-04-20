package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InMemoryStore struct {
	mu sync.RWMutex

	manifests map[uuid.UUID][]byte

	manifestMeta map[uuid.UUID]ManifestMeta

	peers map[uuid.UUID]PeerInfo

	seeders map[uuid.UUID]map[uuid.UUID]struct{}
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		manifests:    make(map[uuid.UUID][]byte),
		manifestMeta: make(map[uuid.UUID]ManifestMeta),
		peers:        make(map[uuid.UUID]PeerInfo),
		seeders:      make(map[uuid.UUID]map[uuid.UUID]struct{}),
	}
}

func (s *InMemoryStore) SaveManifest(id uuid.UUID, name, description string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := make([]byte, len(data))
	copy(stored, data)

	s.manifests[id] = stored
	s.manifestMeta[id] = ManifestMeta{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	return nil
}

func (s *InMemoryStore) GetManifest(id uuid.UUID) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.manifests[id]
	if !ok {
		return nil, fmt.Errorf("manifest %v not found", id)
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (s *InMemoryStore) ListManifests() ([]ManifestMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ManifestMeta, 0, len(s.manifestMeta))
	for _, meta := range s.manifestMeta {
		result = append(result, meta)
	}
	return result, nil
}

func (s *InMemoryStore) RegisterPeer(peerID uuid.UUID, address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.peers[peerID] = PeerInfo{
		ID:       peerID,
		Address:  address,
		LastSeen: time.Now().UTC(),
	}
	return nil
}

func (s *InMemoryStore) GetPeer(peerID uuid.UUID) (PeerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peer, ok := s.peers[peerID]
	if !ok {
		return PeerInfo{}, fmt.Errorf("peer %v not found", peerID)
	}
	return peer, nil
}

func (s *InMemoryStore) AnnounceSeeder(manifestID uuid.UUID, peerID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.manifests[manifestID]; !ok {
		return fmt.Errorf("manifest %v not found", manifestID)
	}
	if _, ok := s.peers[peerID]; !ok {
		return fmt.Errorf("peer %v not found", peerID)
	}

	peer := s.peers[peerID]
	peer.LastSeen = time.Now().UTC()
	s.peers[peerID] = peer

	if _, ok := s.seeders[manifestID]; !ok {
		s.seeders[manifestID] = make(map[uuid.UUID]struct{})
	}
	s.seeders[manifestID][peerID] = struct{}{}

	return nil
}

func (s *InMemoryStore) GetSeeders(manifestID uuid.UUID) ([]PeerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seederIDs, ok := s.seeders[manifestID]
	if !ok {
		return []PeerInfo{}, nil
	}

	result := make([]PeerInfo, 0, len(seederIDs))
	for peerID := range seederIDs {
		if peer, ok := s.peers[peerID]; ok {
			result = append(result, peer)
		}
	}
	return result, nil
}
