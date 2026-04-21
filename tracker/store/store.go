package store

import (
	"time"

	"github.com/google/uuid"
)

type ManifestMeta struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	Embedding   []float32
}

type PeerInfo struct {
	ID       uuid.UUID // id пира
	Address  string    // "ip:port" для соединений пиров между собой (TCP)
	LastSeen time.Time // время последнего запроса от пира
}

type SearchResult struct {
	ID          uuid.UUID
	Name        string
	Description string
	Score       float32
}

// сечас in-memory реализация, потоп sqlite
type TrackerStore interface {
	// манифесты
	SaveManifest(id uuid.UUID, name, description string, embedding []float32, data []byte) error

	GetManifest(id uuid.UUID) ([]byte, error)

	ListManifests() ([]ManifestMeta, error)

	SearchManifests(queryEmbedding []float32, topK int) ([]SearchResult, error)

	// пиры
	RegisterPeer(peerID uuid.UUID, address string) error

	GetPeer(peerID uuid.UUID) (PeerInfo, error)

	// пир-манифест
	AnnounceSeeder(manifestID uuid.UUID, peerID uuid.UUID) error

	// раздающие пиры по манифесту
	GetSeeders(manifestID uuid.UUID) ([]PeerInfo, error)
}
