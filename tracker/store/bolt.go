package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

var (
	bucketManifests    = []byte("manifests")
	bucketManifestMeta = []byte("manifest_meta")
	bucketPeers        = []byte("peers")
	bucketSeeders      = []byte("seeders")
)

type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("bbolt open %q: %w", path, err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketManifests, bucketManifestMeta, bucketPeers, bucketSeeders} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

func (s *BoltStore) SaveManifest(id uuid.UUID, name, description string, embedding []float32, data []byte) error {
	meta := ManifestMeta{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		Embedding:   embedding,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(bucketManifests).Put(id[:], data); err != nil {
			return err
		}
		return tx.Bucket(bucketManifestMeta).Put(id[:], metaBytes)
	})
}

func (s *BoltStore) GetManifest(id uuid.UUID) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketManifests).Get(id[:])
		if v == nil {
			return fmt.Errorf("manifest %v not found", id)
		}
		result = make([]byte, len(v))
		copy(result, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *BoltStore) ListManifests() ([]ManifestMeta, error) {
	var result []ManifestMeta
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketManifestMeta).ForEach(func(_, v []byte) error {
			var meta ManifestMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}
			result = append(result, meta)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *BoltStore) SearchManifests(queryEmbedding []float32, topK int) ([]SearchResult, error) {
	var results []SearchResult
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketManifestMeta).ForEach(func(_, v []byte) error {
			var meta ManifestMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				return fmt.Errorf("unmarshal meta: %w", err)
			}
			if meta.Embedding == nil {
				return nil
			}
			score := cosineSimilarity(queryEmbedding, meta.Embedding)
			results = append(results, SearchResult{
				ID:          meta.ID,
				Name:        meta.Name,
				Description: meta.Description,
				Score:       score,
				CosineScore: score,
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *BoltStore) RegisterPeer(peerID uuid.UUID, address string) error {
	peer := PeerInfo{
		ID:       peerID,
		Address:  address,
		LastSeen: time.Now().UTC(),
	}
	peerBytes, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("marshal peer: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketPeers).Put(peerID[:], peerBytes)
	})
}

func (s *BoltStore) GetPeer(peerID uuid.UUID) (PeerInfo, error) {
	var peer PeerInfo
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketPeers).Get(peerID[:])
		if v == nil {
			return fmt.Errorf("peer %v not found", peerID)
		}
		return json.Unmarshal(v, &peer)
	})
	if err != nil {
		return PeerInfo{}, err
	}
	return peer, nil
}

func seederKey(manifestID, peerID uuid.UUID) []byte {
	key := make([]byte, 32)
	copy(key[:16], manifestID[:])
	copy(key[16:], peerID[:])
	return key
}

func (s *BoltStore) AnnounceSeeder(manifestID uuid.UUID, peerID uuid.UUID) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket(bucketManifests).Get(manifestID[:]) == nil {
			return fmt.Errorf("manifest %v not found", manifestID)
		}
		peerBytes := tx.Bucket(bucketPeers).Get(peerID[:])
		if peerBytes == nil {
			return fmt.Errorf("peer %v not found", peerID)
		}

		var peer PeerInfo
		if err := json.Unmarshal(peerBytes, &peer); err != nil {
			return fmt.Errorf("unmarshal peer: %w", err)
		}
		peer.LastSeen = time.Now().UTC()
		updated, err := json.Marshal(peer)
		if err != nil {
			return fmt.Errorf("marshal peer: %w", err)
		}
		if err := tx.Bucket(bucketPeers).Put(peerID[:], updated); err != nil {
			return err
		}

		return tx.Bucket(bucketSeeders).Put(seederKey(manifestID, peerID), []byte{})
	})
}

func (s *BoltStore) GetSeeders(manifestID uuid.UUID) ([]PeerInfo, error) {
	var result []PeerInfo
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucketSeeders).Cursor()
		prefix := manifestID[:]
		peersBucket := tx.Bucket(bucketPeers)
		for k, _ := c.Seek(prefix); len(k) == 32 && string(k[:16]) == string(prefix); k, _ = c.Next() {
			var peerID uuid.UUID
			copy(peerID[:], k[16:])
			peerBytes := peersBucket.Get(peerID[:])
			if peerBytes == nil {
				continue
			}
			var peer PeerInfo
			if err := json.Unmarshal(peerBytes, &peer); err != nil {
				return fmt.Errorf("unmarshal peer: %w", err)
			}
			result = append(result, peer)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []PeerInfo{}
	}
	return result, nil
}
