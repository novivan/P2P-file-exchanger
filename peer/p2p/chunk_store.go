package p2p

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type InMemoryChunkStore struct {
	mu     sync.RWMutex
	chunks map[uuid.UUID]map[uint32][]byte
}

func NewInMemoryChunkStore() *InMemoryChunkStore {
	return &InMemoryChunkStore{
		chunks: make(map[uuid.UUID]map[uint32][]byte),
	}
}

func (s *InMemoryChunkStore) LoadFiles(manifestID uuid.UUID, files [][]byte, chunkLen int64) error {
	if chunkLen <= 0 {
		return fmt.Errorf("LoadFiles: chunkLen must be > 0, got %d", chunkLen)
	}

	var sumLen int64
	for _, f := range files {
		sumLen += int64(len(f))
	}
	if sumLen == 0 {
		return fmt.Errorf("LoadFiles: no data to load")
	}

	readByte := func(pos int64) byte {
		for _, f := range files {
			flen := int64(len(f))
			if pos < flen {
				return f[pos]
			}
			pos -= flen
		}
		return 0
	}

	chunksAmount := (sumLen + chunkLen - 1) / chunkLen
	chunkMap := make(map[uint32][]byte, chunksAmount)

	var globalPos int64
	for idx := int64(0); idx < chunksAmount; idx++ {
		remaining := sumLen - globalPos
		curLen := chunkLen
		if remaining < chunkLen {
			curLen = remaining
		}

		chunk := make([]byte, curLen)
		for i := int64(0); i < curLen; i++ {
			chunk[i] = readByte(globalPos)
			globalPos++
		}
		chunkMap[uint32(idx)] = chunk
	}

	s.mu.Lock()
	s.chunks[manifestID] = chunkMap
	s.mu.Unlock()

	return nil
}

func (s *InMemoryChunkStore) GetChunk(manifestID uuid.UUID, chunkIndex uint32) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chunkMap, ok := s.chunks[manifestID]
	if !ok {
		return nil, fmt.Errorf("GetChunk: manifest %v not found", manifestID)
	}

	data, ok := chunkMap[chunkIndex]
	if !ok {
		return nil, fmt.Errorf("GetChunk: chunk %d not found in manifest %v", chunkIndex, manifestID)
	}

	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}
