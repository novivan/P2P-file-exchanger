package store

import (
	"fmt"
	"math"
	"sort"
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

func (s *InMemoryStore) SaveManifest(id uuid.UUID, name, description string, embedding []float32, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := make([]byte, len(data))
	copy(stored, data)

	var storedEmbedding []float32
	if embedding != nil {
		storedEmbedding = make([]float32, len(embedding))
		copy(storedEmbedding, embedding)
	}

	s.manifests[id] = stored
	s.manifestMeta[id] = ManifestMeta{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Embedding:   storedEmbedding,
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

func (s *InMemoryStore) SearchManifests(queryEmbedding []float32, topK int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult
	for id, meta := range s.manifestMeta {
		if meta.Embedding == nil {
			continue
		}
		score := cosineSimilarity(queryEmbedding, meta.Embedding)
		results = append(results, SearchResult{
			ID:          id,
			Name:        meta.Name,
			Description: meta.Description,
			Score:       score, // Переопределяется потом, если вызывается llm'ка
			CosineScore: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	var dot, normA, normB float32
	for i := 0; i < n; i++ {
		var ai, bi float32
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
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
