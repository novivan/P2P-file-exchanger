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
}

type PeerInfo struct {
	ID       uuid.UUID // id пира
	Address  string    // "ip:port" для соединений пиров между собой (TCP)
	LastSeen time.Time // время последнего запроса от пира
}

// сечас in-memory реализация, потоп sqlite
type TrackerStore interface {
	// манифесты
	SaveManifest(id uuid.UUID, name, description string, data []byte) error

	GetManifest(id uuid.UUID) ([]byte, error)

	ListManifests() ([]ManifestMeta, error)

	// пиры
	RegisterPeer(peerID uuid.UUID, address string) error

	GetPeer(peerID uuid.UUID) (PeerInfo, error)

	// пир-манифест
	AnnounceSeeder(manifestID uuid.UUID, peerID uuid.UUID) error

	// раздающие пиры по манифесту
	GetSeeders(manifestID uuid.UUID) ([]PeerInfo, error)
}
