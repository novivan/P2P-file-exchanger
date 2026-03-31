package codec

import (
	"crypto/sha1"
	"testing"
)

func makeBytes(length int, val byte) []byte {
	b := make([]byte, length)
	for i := range b {
		b[i] = val
	}
	return b
}

func sha1Of(data []byte) [sha1.Size]byte {
	return sha1.Sum(data)
}

// --- tests ---

func TestEncodeNoFiles(t *testing.T) {
	c := &Codec{}
	_, err := c.Encode([][]byte{})
	if err == nil {
		t.Fatal("expected error for empty files list, got nil")
	}
}

func TestEncodeAllEmpty(t *testing.T) {
	c := &Codec{}
	_, err := c.Encode([][]byte{{}, {}})
	if err == nil {
		t.Fatal("expected error for all-empty files, got nil")
	}
}

func TestEncodeSingleSmallFile(t *testing.T) {
	c := &Codec{}
	data := []byte("hello, world!")
	hashes, err := c.Encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hashes) != 13 {
		t.Fatalf("expected 13 chunks, got %d", len(hashes))
	}
	for i, b := range data {
		expected := sha1Of([]byte{b})
		if hashes[i] != expected {
			t.Errorf("chunk %d: hash mismatch:\n  got  %x\n  want %x", i, hashes[i], expected)
		}
	}
}

func TestEncodeExactlyOneChunk(t *testing.T) {
	c := &Codec{}
	data := makeBytes(1024, 0xAB)
	hashes, err := c.Encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hashes) != 1024 {
		t.Fatalf("expected 1024 chunks, got %d", len(hashes))
	}
	for i, b := range data {
		expected := sha1Of([]byte{b})
		if hashes[i] != expected {
			t.Errorf("chunk %d: hash mismatch:\n  got  %x\n  want %x", i, hashes[i], expected)
		}
	}
}

func TestEncodeManyChunks(t *testing.T) {
	c := &Codec{}
	data := makeBytes(3000, 0x01)
	hashes, err := c.Encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chunkLen := calcChunkLen(3000)
	expectedChunks := (3000 + chunkLen - 1) / chunkLen
	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

	// проверяем каждый хеш вручную
	for i := int64(0); i < expectedChunks; i++ {
		start := i * chunkLen
		end := start + chunkLen
		if end > 3000 {
			end = 3000
		}
		expected := sha1Of(data[start:end])
		if hashes[i] != expected {
			t.Errorf("chunk %d: hash mismatch:\n  got  %x\n  want %x", i, hashes[i], expected)
		}
	}
}

func TestEncodeMultipleFiles(t *testing.T) {
	c := &Codec{}
	file1 := []byte{0x01, 0x02, 0x03}
	file2 := []byte{0x04, 0x05, 0x06}

	hashes, err := c.Encode([][]byte{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	combined := append(file1, file2...)
	chunkLen := calcChunkLen(int64(len(combined)))
	expectedChunks := (int64(len(combined)) + chunkLen - 1) / chunkLen

	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

	for i := int64(0); i < expectedChunks; i++ {
		start := i * chunkLen
		end := start + chunkLen
		if end > int64(len(combined)) {
			end = int64(len(combined))
		}
		expected := sha1Of(combined[start:end])
		if hashes[i] != expected {
			t.Errorf("chunk %d: hash mismatch:\n  got  %x\n  want %x", i, hashes[i], expected)
		}
	}
}

func TestEncodeChunkBoundary(t *testing.T) {
	c := &Codec{}
	half := 1024
	file1 := makeBytes(half, 0xAA)
	file2 := makeBytes(half, 0xBB)

	hashes, err := c.Encode([][]byte{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	combined := append(file1, file2...)
	chunkLen := calcChunkLen(int64(len(combined)))
	expectedChunks := (int64(len(combined)) + chunkLen - 1) / chunkLen

	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

	for i := int64(0); i < expectedChunks; i++ {
		start := i * chunkLen
		end := start + chunkLen
		if end > int64(len(combined)) {
			end = int64(len(combined))
		}
		expected := sha1Of(combined[start:end])
		if hashes[i] != expected {
			t.Errorf("chunk %d: hash mismatch:\n  got  %x\n  want %x", i, hashes[i], expected)
		}
	}
}

func TestEncodeLastChunkShorter(t *testing.T) {
	c := &Codec{}
	data := makeBytes(1025, 0xFF)
	hashes, err := c.Encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chunkLen := calcChunkLen(1025)
	expectedChunks := (1025 + chunkLen - 1) / chunkLen
	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

	lastChunk := data[int64(len(data))-1025%chunkLen:]
	if 1025%chunkLen == 0 {
		lastChunk = data[int64(len(data))-chunkLen:]
	}
	expectedLastHash := sha1Of(lastChunk)
	if hashes[len(hashes)-1] != expectedLastHash {
		t.Errorf("last chunk hash mismatch:\n  got  %x\n  want %x", hashes[len(hashes)-1], expectedLastHash)
	}
}

func TestCalcChunkLen(t *testing.T) {
	cases := []struct {
		sumlen   int64
		expected int64
	}{
		{0, 1},
		{1, 1},
		{1023, 1},
		{1024, 1},
		{1025, 2},
		{2048, 2},
		{2049, 4},
		{3000, 4},
		{4096, 4},
		{4097, 8},
		{1024 * 1024, 1024},
	}
	for _, tc := range cases {
		got := calcChunkLen(tc.sumlen)
		if got != tc.expected {
			t.Errorf("calcChunkLen(%d) = %d, want %d", tc.sumlen, got, tc.expected)
		}
	}
}
