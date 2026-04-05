package codec

import (
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// строим реалбьный манифест файл из реального файла
func TestIntegrationBuildManifestFromFile(t *testing.T) {
	filePath := filepath.Join("testdata", "sample.txt")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read test file %q: %v", filePath, err)
	}
	if len(fileData) == 0 {
		t.Fatalf("test file %q is empty", filePath)
	}

	c := &Codec{}
	manifestID := uuid.New()
	peerID := uuid.New()

	m, err := c.BuildManifest(
		manifestID,
		[][]byte{fileData},
		nil,
		filepath.Base(filePath),
		[]string{"http://localhost:8080"},
		"integration test manifest",
		peerID,
	)
	if err != nil {
		t.Fatalf("BuildManifest error: %v", err)
	}

	if m.ID != manifestID {
		t.Errorf("ID: got %v, want %v", m.ID, manifestID)
	}
	if m.Info.Name != "sample.txt" {
		t.Errorf("Name: got %q, want %q", m.Info.Name, "sample.txt")
	}
	if m.Info.Length != int64(len(fileData)) {
		t.Errorf("Length: got %d, want %d", m.Info.Length, len(fileData))
	}
	if m.Info.PieceLength <= 0 {
		t.Errorf("PieceLength: got %d, expected > 0", m.Info.PieceLength)
	}
	if len(m.Info.Pieces) == 0 {
		t.Fatal("Pieces: expected non-empty")
	}

	chunkLen := m.Info.PieceLength
	for i, h := range m.Info.Pieces {
		start := int64(i) * chunkLen
		end := start + chunkLen
		if end > int64(len(fileData)) {
			end = int64(len(fileData))
		}
		expected := sha1.Sum(fileData[start:end])
		if h != expected {
			t.Errorf("Pieces[%d]: hash mismatch\n  got  %x\n  want %x", i, h, expected)
		}
	}

	t.Logf("file size: %d bytes, chunkLen: %d, chunks: %d",
		len(fileData), chunkLen, len(m.Info.Pieces))

	encoded, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("Marshal: got empty output")
	}
	t.Logf("marshalled manifest size: %d bytes", len(encoded))

	decoded, err := Unmarshal(encoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != m.ID {
		t.Errorf("roundtrip ID: got %v, want %v", decoded.ID, m.ID)
	}
	if decoded.Info.Name != m.Info.Name {
		t.Errorf("roundtrip Name: got %q, want %q", decoded.Info.Name, m.Info.Name)
	}
	if decoded.Info.Length != m.Info.Length {
		t.Errorf("roundtrip Length: got %d, want %d", decoded.Info.Length, m.Info.Length)
	}
	if decoded.Info.PieceLength != m.Info.PieceLength {
		t.Errorf("roundtrip PieceLength: got %d, want %d", decoded.Info.PieceLength, m.Info.PieceLength)
	}
	if decoded.Comment != m.Comment {
		t.Errorf("roundtrip Comment: got %q, want %q", decoded.Comment, m.Comment)
	}
	if decoded.CreatedBy != m.CreatedBy {
		t.Errorf("roundtrip CreatedBy: got %v, want %v", decoded.CreatedBy, m.CreatedBy)
	}
	if decoded.CreationDate.Unix() != m.CreationDate.Unix() {
		t.Errorf("roundtrip CreationDate: got %v, want %v", decoded.CreationDate, m.CreationDate)
	}
	if len(decoded.AnnounceList) != len(m.AnnounceList) {
		t.Fatalf("roundtrip AnnounceList len: got %d, want %d", len(decoded.AnnounceList), len(m.AnnounceList))
	}
	if decoded.AnnounceList[0] != m.AnnounceList[0] {
		t.Errorf("roundtrip AnnounceList[0]: got %q, want %q", decoded.AnnounceList[0], m.AnnounceList[0])
	}
	if len(decoded.Info.Pieces) != len(m.Info.Pieces) {
		t.Fatalf("roundtrip Pieces count: got %d, want %d", len(decoded.Info.Pieces), len(m.Info.Pieces))
	}
	for i := range m.Info.Pieces {
		if decoded.Info.Pieces[i] != m.Info.Pieces[i] {
			t.Errorf("roundtrip Pieces[%d]: got %x, want %x", i, decoded.Info.Pieces[i], m.Info.Pieces[i])
		}
	}

	// сохраняем манифест на диск рядом с исходным файлом (посмотреть)
	outPath := filepath.Join("testdata", "sample.manifest")
	if err := os.WriteFile(outPath, encoded, 0644); err != nil {
		t.Logf("warning: could not write manifest file: %v", err)
	} else {
		t.Logf("manifest written to %s", outPath)
	}
}
