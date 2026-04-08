package p2p

import (
	"crypto/sha1"
	"testing"

	"github.com/google/uuid"
)

func NewTestInMemoryChunkStore() *InMemoryChunkStore {
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

func TestRequestSingleChunk(t *testing.T) {
	manifestID := uuid.New()
	chunkData := []byte("hello from chunk zero")
	chunkIndex := uint32(0)

	store := NewTestInMemoryChunkStore()
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

	if string(received) != string(chunkData) {
		t.Errorf("chunk data mismatch:\n  got  %q\n  want %q", received, chunkData)
	}
}

func TestRequestChunkVerifySHA1(t *testing.T) {
	manifestID := uuid.New()
	chunkData := []byte("this chunk will be verified by sha1 hash")
	chunkIndex := uint32(3)

	expectedHash := sha1.Sum(chunkData)

	store := NewTestInMemoryChunkStore()
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

	actualHash := sha1.Sum(received)
	if actualHash != expectedHash {
		t.Errorf("SHA-1 verification failed:\n  got  %x\n  want %x", actualHash, expectedHash)
	}
}

// TestRequestChunkNotFound — запрос несуществующего чанка должен вернуть ошибку.
func TestRequestChunkNotFound(t *testing.T) {
	store := NewTestInMemoryChunkStore()

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

	store := NewTestInMemoryChunkStore()
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

	store := NewTestInMemoryChunkStore()
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
	chunkData := make([]byte, 256)
	for i := range chunkData {
		chunkData[i] = byte(i)
	}
	expectedHash := sha1.Sum(chunkData)

	store := NewTestInMemoryChunkStore()
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

// тесты LoadFiles
func TestLoadFiles_SingleFile(t *testing.T) {
	data := []byte("ABCDEFGHI")
	manifestID := uuid.New()

	store := NewInMemoryChunkStore()
	if err := store.LoadFiles(manifestID, [][]byte{data}, 4); err != nil {
		t.Fatalf("LoadFiles error: %v", err)
	}

	c0, err := store.GetChunk(manifestID, 0)
	if err != nil {
		t.Fatalf("GetChunk(0): %v", err)
	}
	if string(c0) != "ABCD" {
		t.Errorf("chunk 0: got %q, want %q", c0, "ABCD")
	}

	c1, err := store.GetChunk(manifestID, 1)
	if err != nil {
		t.Fatalf("GetChunk(1): %v", err)
	}
	if string(c1) != "EFGH" {
		t.Errorf("chunk 1: got %q, want %q", c1, "EFGH")
	}

	c2, err := store.GetChunk(manifestID, 2)
	if err != nil {
		t.Fatalf("GetChunk(2): %v", err)
	}
	if string(c2) != "I" {
		t.Errorf("chunk 2: got %q, want %q", c2, "I")
	}

	_, err = store.GetChunk(manifestID, 3)
	if err == nil {
		t.Error("expected error for chunk 3, got nil")
	}
}

func TestLoadFiles_MultipleFiles(t *testing.T) {
	files := [][]byte{[]byte("ABC"), []byte("DE")}
	manifestID := uuid.New()

	store := NewInMemoryChunkStore()
	if err := store.LoadFiles(manifestID, files, 2); err != nil {
		t.Fatalf("LoadFiles error: %v", err)
	}

	expected := []string{"AB", "CD", "E"}
	for i, want := range expected {
		got, err := store.GetChunk(manifestID, uint32(i))
		if err != nil {
			t.Fatalf("GetChunk(%d): %v", i, err)
		}
		if string(got) != want {
			t.Errorf("chunk %d: got %q, want %q", i, got, want)
		}
	}
}

func TestLoadFiles_ExactChunkBoundary(t *testing.T) {
	data := []byte("ABCD")
	manifestID := uuid.New()

	store := NewInMemoryChunkStore()
	if err := store.LoadFiles(manifestID, [][]byte{data}, 2); err != nil {
		t.Fatalf("LoadFiles error: %v", err)
	}

	c0, _ := store.GetChunk(manifestID, 0)
	c1, _ := store.GetChunk(manifestID, 1)
	if string(c0) != "AB" {
		t.Errorf("chunk 0: got %q, want %q", c0, "AB")
	}
	if string(c1) != "CD" {
		t.Errorf("chunk 1: got %q, want %q", c1, "CD")
	}
	_, err := store.GetChunk(manifestID, 2)
	if err == nil {
		t.Error("expected error for chunk 2, got nil")
	}
}

func TestLoadFiles_InvalidChunkLen(t *testing.T) {
	store := NewInMemoryChunkStore()
	err := store.LoadFiles(uuid.New(), [][]byte{[]byte("data")}, 0)
	if err == nil {
		t.Error("expected error for chunkLen=0, got nil")
	}
	err = store.LoadFiles(uuid.New(), [][]byte{[]byte("data")}, -1)
	if err == nil {
		t.Error("expected error for chunkLen=-1, got nil")
	}
}

func TestLoadFiles_EmptyData(t *testing.T) {
	store := NewInMemoryChunkStore()
	err := store.LoadFiles(uuid.New(), [][]byte{{}}, 4)
	if err == nil {
		t.Error("expected error for empty files, got nil")
	}
}

func TestLoadFiles_GetChunkReturnsCopy(t *testing.T) {
	data := []byte("HELLO")
	manifestID := uuid.New()

	store := NewInMemoryChunkStore()
	store.LoadFiles(manifestID, [][]byte{data}, 5)

	got, _ := store.GetChunk(manifestID, 0)
	got[0] = 'X' // мутируем возвращённую копию

	got2, _ := store.GetChunk(manifestID, 0)
	if got2[0] != 'H' {
		t.Errorf("GetChunk should return a copy; store was mutated: got %q", got2)
	}
}

func TestLoadFiles_TwoManifests(t *testing.T) {
	store := NewInMemoryChunkStore()
	idA := uuid.New()
	idB := uuid.New()

	store.LoadFiles(idA, [][]byte{[]byte("AAA")}, 3)
	store.LoadFiles(idB, [][]byte{[]byte("BBB")}, 3)

	a, _ := store.GetChunk(idA, 0)
	b, _ := store.GetChunk(idB, 0)

	if string(a) != "AAA" {
		t.Errorf("manifest A chunk: got %q, want %q", a, "AAA")
	}
	if string(b) != "BBB" {
		t.Errorf("manifest B chunk: got %q, want %q", b, "BBB")
	}
}

func TestLoadFiles_SeederServesLoadedChunks(t *testing.T) {
	fileData := []byte("Hello, P2P world!")
	manifestID := uuid.New()
	chunkLen := int64(4)

	store := NewInMemoryChunkStore()
	if err := store.LoadFiles(manifestID, [][]byte{fileData}, chunkLen); err != nil {
		t.Fatalf("LoadFiles: %v", err)
	}

	seeder, err := NewSeeder(":0", store)
	if err != nil {
		t.Fatalf("NewSeeder: %v", err)
	}
	defer seeder.Close()
	go seeder.Serve()

	addr := seeder.Addr().String()

	var reconstructed []byte
	for i := uint32(0); ; i++ {
		chunk, err := RequestChunk(addr, manifestID, i)
		if err != nil {
			break
		}
		reconstructed = append(reconstructed, chunk...)
	}

	if string(reconstructed) != string(fileData) {
		t.Errorf("reconstructed file mismatch:\n  got  %q\n  want %q", reconstructed, fileData)
	}
}
