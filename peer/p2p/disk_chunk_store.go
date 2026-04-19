package p2p

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
)

type DiskChunkStore struct {
	mu        sync.RWMutex
	manifests map[uuid.UUID]diskManifest
}

type diskManifest struct {
	filePaths []string
	fileLens  []int64
	chunkLen  int64
	totalLen  int64
}

func NewDiskChunkStore() *DiskChunkStore {
	return &DiskChunkStore{
		manifests: make(map[uuid.UUID]diskManifest),
	}
}

func (s *DiskChunkStore) Register(manifestID uuid.UUID, filePaths []string, chunkLen int64) error {
	if chunkLen <= 0 {
		return fmt.Errorf("DiskChunkStore.Register: chunkLen must be > 0, got %d", chunkLen)
	}
	if len(filePaths) == 0 {
		return fmt.Errorf("DiskChunkStore.Register: no files provided")
	}

	fileLens := make([]int64, len(filePaths))
	var totalLen int64
	for i, p := range filePaths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("DiskChunkStore.Register: stat %q: %w", p, err)
		}
		if info.IsDir() {
			return fmt.Errorf("DiskChunkStore.Register: %q is a directory", p)
		}
		fileLens[i] = info.Size()
		totalLen += info.Size()
	}
	if totalLen == 0 {
		return fmt.Errorf("DiskChunkStore.Register: total data length is 0")
	}

	s.mu.Lock()
	s.manifests[manifestID] = diskManifest{
		filePaths: append([]string(nil), filePaths...),
		fileLens:  fileLens,
		chunkLen:  chunkLen,
		totalLen:  totalLen,
	}
	s.mu.Unlock()

	return nil
}

func (s *DiskChunkStore) Unregister(manifestID uuid.UUID) {
	s.mu.Lock()
	delete(s.manifests, manifestID)
	s.mu.Unlock()
}

func (s *DiskChunkStore) Has(manifestID uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.manifests[manifestID]
	return ok
}

func (s *DiskChunkStore) GetChunk(manifestID uuid.UUID, chunkIndex uint32) ([]byte, error) {
	s.mu.RLock()
	m, ok := s.manifests[manifestID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("DiskChunkStore.GetChunk: manifest %v not found", manifestID)
	}

	globalOffset := int64(chunkIndex) * m.chunkLen
	if globalOffset >= m.totalLen {
		return nil, fmt.Errorf("DiskChunkStore.GetChunk: chunk %d out of range (totalLen=%d)", chunkIndex, m.totalLen)
	}
	remaining := m.totalLen - globalOffset
	curLen := m.chunkLen
	if remaining < curLen {
		curLen = remaining
	}

	buf := make([]byte, curLen)

	cursor := globalOffset
	fileIdx := 0
	for fileIdx < len(m.fileLens) && cursor >= m.fileLens[fileIdx] {
		cursor -= m.fileLens[fileIdx]
		fileIdx++
	}

	var written int64
	for written < curLen {
		if fileIdx >= len(m.filePaths) {
			return nil, fmt.Errorf("DiskChunkStore.GetChunk: ran out of files before chunk filled")
		}
		f, err := os.Open(m.filePaths[fileIdx])
		if err != nil {
			return nil, fmt.Errorf("DiskChunkStore.GetChunk: open %q: %w", m.filePaths[fileIdx], err)
		}

		need := curLen - written
		available := m.fileLens[fileIdx] - cursor
		readLen := need
		if available < readLen {
			readLen = available
		}

		n, err := f.ReadAt(buf[written:written+readLen], cursor)
		f.Close()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("DiskChunkStore.GetChunk: read %q: %w", m.filePaths[fileIdx], err)
		}
		if int64(n) != readLen {
			return nil, fmt.Errorf("DiskChunkStore.GetChunk: short read from %q: got %d want %d", m.filePaths[fileIdx], n, readLen)
		}

		written += readLen
		cursor = 0
		fileIdx++
	}

	return buf, nil
}
