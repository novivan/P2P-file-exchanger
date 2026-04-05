package p2p

import (
	"crypto/sha1"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// ---- InMemoryChunkStore — простое хранилище чанков в памяти для тестов ----

// InMemoryChunkStore хранит чанки в памяти: map[manifestID][chunkIndex] = data
type InMemoryChunkStore struct {
	chunks map[uuid.UUID]map[uint32][]byte
}

func NewInMemoryChunkStore() *InMemoryChunkStore {
	return &InMemoryChunkStore{
		chunks: make(map[uuid.UUID]map[uint32][]byte),
	}
}

func (s *InMemoryChunkStore) AddChunk(manifestID uuid.UUID, chunkIndex uint32, data []byte) {
	if _, ok := s.chunks[manifestID]; !ok {
		s.chunks[manifestID] = make(map[uint32][]byte)
	}
	s.chunks[manifestID][chunkIndex] = data
}

func (s *InMemoryChunkStore) GetChunk(manifestID uuid.UUID, chunkIndex uint32) ([]byte, error) {
	manifest, ok := s.chunks[manifestID]
	if !ok {
		return nil, fmt.Errorf("GetChunk: manifest %v not found", manifestID)
	}
	chunk, ok := manifest[chunkIndex]
	if !ok {
		return nil, fmt.Errorf("GetChunk: chunk %d not found in manifest %v", chunkIndex, manifestID)
	}
	return chunk, nil
}

// ---- тесты -----------------------------------------------------------------

// TestRequestSingleChunk — базовый тест: seeder отдаёт один чанк, leecher получает его.
func TestRequestSingleChunk(t *testing.T) {
	manifestID := uuid.New()
	chunkData := []byte("hello from chunk zero")
	chunkIndex := uint32(0)

	store := NewInMemoryChunkStore()
	store.AddChunk(manifestID, chunkIndex, chunkData)

	// запускаем seeder на случайном порту (":0" — ОС выберет свободный)
	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()

	// запускаем сервер в горутине
	go seeder.Serve()

	// leecher запрашивает чанк
	addr := seeder.Addr().String()
	received, err := RequestChunk(addr, manifestID, chunkIndex)
	if err != nil {
		t.Fatalf("RequestChunk error: %v", err)
	}

	if string(received) != string(chunkData) {
		t.Errorf("chunk data mismatch:\n  got  %q\n  want %q", received, chunkData)
	}
}

// TestRequestChunkVerifySHA1 — leecher получает чанк и верифицирует его SHA-1 хеш.
// Это ключевая проверка целостности данных в P2P-обмене.
func TestRequestChunkVerifySHA1(t *testing.T) {
	manifestID := uuid.New()
	chunkData := []byte("this chunk will be verified by sha1 hash")
	chunkIndex := uint32(3)

	// seeder знает хеш заранее (из манифеста)
	expectedHash := sha1.Sum(chunkData)

	store := NewInMemoryChunkStore()
	store.AddChunk(manifestID, chunkIndex, chunkData)

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()
	received, err := RequestChunk(addr, manifestID, chunkIndex)
	if err != nil {
		t.Fatalf("RequestChunk error: %v", err)
	}

	// leecher верифицирует хеш полученного чанка
	actualHash := sha1.Sum(received)
	if actualHash != expectedHash {
		t.Errorf("SHA-1 verification failed:\n  got  %x\n  want %x", actualHash, expectedHash)
	}
}

// TestRequestChunkNotFound — запрос несуществующего чанка должен вернуть ошибку.
func TestRequestChunkNotFound(t *testing.T) {
	store := NewInMemoryChunkStore()
	// хранилище пустое — ни одного чанка

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()
	_, err = RequestChunk(addr, uuid.New(), 0)
	if err == nil {
		t.Fatal("expected error for missing chunk, got nil")
	}
}

// TestRequestMultipleChunksSequentially — последовательный запрос нескольких чанков.
// Каждый чанк верифицируется по SHA-1.
func TestRequestMultipleChunksSequentially(t *testing.T) {
	manifestID := uuid.New()
	chunks := [][]byte{
		[]byte("chunk number zero"),
		[]byte("chunk number one"),
		[]byte("chunk number two"),
	}

	store := NewInMemoryChunkStore()
	for i, data := range chunks {
		store.AddChunk(manifestID, uint32(i), data)
	}

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()

	for i, expected := range chunks {
		received, err := RequestChunk(addr, manifestID, uint32(i))
		if err != nil {
			t.Fatalf("RequestChunk(%d) error: %v", i, err)
		}

		// проверяем данные
		if string(received) != string(expected) {
			t.Errorf("chunk %d data mismatch:\n  got  %q\n  want %q", i, received, expected)
		}

		// верифицируем SHA-1
		expectedHash := sha1.Sum(expected)
		actualHash := sha1.Sum(received)
		if actualHash != expectedHash {
			t.Errorf("chunk %d SHA-1 mismatch:\n  got  %x\n  want %x", i, actualHash, expectedHash)
		}
	}
}

// TestRequestChunksFromTwoManifests — два манифеста на одном seeder'е,
// leecher запрашивает чанки из обоих.
func TestRequestChunksFromTwoManifests(t *testing.T) {
	manifestA := uuid.New()
	manifestB := uuid.New()

	store := NewInMemoryChunkStore()
	store.AddChunk(manifestA, 0, []byte("manifest A, chunk 0"))
	store.AddChunk(manifestB, 0, []byte("manifest B, chunk 0"))

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()

	dataA, err := RequestChunk(addr, manifestA, 0)
	if err != nil {
		t.Fatalf("RequestChunk(manifestA) error: %v", err)
	}
	if string(dataA) != "manifest A, chunk 0" {
		t.Errorf("manifestA chunk: got %q", dataA)
	}

	dataB, err := RequestChunk(addr, manifestB, 0)
	if err != nil {
		t.Fatalf("RequestChunk(manifestB) error: %v", err)
	}
	if string(dataB) != "manifest B, chunk 0" {
		t.Errorf("manifestB chunk: got %q", dataB)
	}
}

// TestRequestBinaryChunk — чанк с бинарными данными (не только ASCII).
func TestRequestBinaryChunk(t *testing.T) {
	manifestID := uuid.New()
	// бинарные данные: все возможные байты 0x00..0xFF
	chunkData := make([]byte, 256)
	for i := range chunkData {
		chunkData[i] = byte(i)
	}
	expectedHash := sha1.Sum(chunkData)

	store := NewInMemoryChunkStore()
	store.AddChunk(manifestID, 0, chunkData)

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder error: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()
	received, err := RequestChunk(addr, manifestID, 0)
	if err != nil {
		t.Fatalf("RequestChunk error: %v", err)
	}

	actualHash := sha1.Sum(received)
	if actualHash != expectedHash {
		t.Errorf("binary chunk SHA-1 mismatch:\n  got  %x\n  want %x", actualHash, expectedHash)
	}
	if len(received) != 256 {
		t.Errorf("binary chunk length: got %d, want 256", len(received))
	}
}
