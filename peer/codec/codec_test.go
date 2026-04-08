package codec

import (
	"crypto/sha1"
	"testing"
	"time"

	"github.com/google/uuid"
)

// makeBytes создаёт срез байт заданной длины, заполненный значением val
func makeBytes(length int, val byte) []byte {
	b := make([]byte, length)
	for i := range b {
		b[i] = val
	}
	return b
}

// sha1Of возвращает SHA-1 хеш переданных байт
func sha1Of(data []byte) [sha1.Size]byte {
	return sha1.Sum(data)
}

func TestEncodeNoFiles(t *testing.T) {
	c := &Codec{}
	_, _, err := c.encode([][]byte{})
	if err == nil {
		t.Fatal("expected error for empty files list, got nil")
	}
}

func TestEncodeAllEmpty(t *testing.T) {
	c := &Codec{}
	_, _, err := c.encode([][]byte{{}, {}})
	if err == nil {
		t.Fatal("expected error for all-empty files, got nil")
	}
}

func TestEncodeSingleSmallFile(t *testing.T) {
	c := &Codec{}
	data := []byte("hello, world!")
	hashes, _, err := c.encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 13 байт / chunkLen=1 = 13 чанков
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
	hashes, _, err := c.encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1024 байта / chunkLen=1 = 1024 чанка
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
	hashes, chunkLen, err := c.encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedChunks := (int64(3000) + chunkLen - 1) / chunkLen
	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

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

	hashes, chunkLen, err := c.encode([][]byte{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	combined := append(file1, file2...)
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
	file1 := makeBytes(1024, 0xAA)
	file2 := makeBytes(1024, 0xBB)

	hashes, chunkLen, err := c.encode([][]byte{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	combined := append(file1, file2...)
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
	hashes, chunkLen, err := c.encode([][]byte{data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedChunks := (int64(1025) + chunkLen - 1) / chunkLen
	if int64(len(hashes)) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(hashes))
	}

	lastStart := (expectedChunks - 1) * chunkLen
	expectedLastHash := sha1Of(data[lastStart:])
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

func TestBuildManifestSingleFile(t *testing.T) {
	c := &Codec{}
	data := []byte("hello, world!")
	manifestID := uuid.New()
	peerID := uuid.New()

	m, err := c.BuildManifest(
		manifestID,
		[][]byte{data},
		nil,
		"test.txt",
		[]string{"http://tracker:8080"},
		"test comment",
		peerID,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.ID != manifestID {
		t.Errorf("ID: got %v, want %v", m.ID, manifestID)
	}
	if m.Info.Name != "test.txt" {
		t.Errorf("Name: got %q, want %q", m.Info.Name, "test.txt")
	}
	if m.Info.Length != int64(len(data)) {
		t.Errorf("Length: got %d, want %d", m.Info.Length, len(data))
	}
	if len(m.Info.Files) != 0 {
		t.Errorf("Files: expected empty for single file, got %d entries", len(m.Info.Files))
	}
	if len(m.AnnounceList) != 1 || m.AnnounceList[0] != "http://tracker:8080" {
		t.Errorf("AnnounceList: got %v", m.AnnounceList)
	}
	if m.Comment != "test comment" {
		t.Errorf("Comment: got %q, want %q", m.Comment, "test comment")
	}
	if m.CreatedBy != peerID {
		t.Errorf("CreatedBy: got %v, want %v", m.CreatedBy, peerID)
	}
	if m.Info.PieceLength <= 0 {
		t.Errorf("PieceLength: got %d, expected > 0", m.Info.PieceLength)
	}
	if len(m.Info.Pieces) == 0 {
		t.Error("Pieces: expected non-empty")
	}
	if time.Since(m.CreationDate) > 5*time.Second {
		t.Errorf("CreationDate: too old: %v", m.CreationDate)
	}
}

func TestBuildManifestMultipleFiles(t *testing.T) {
	c := &Codec{}
	file1 := []byte("first file content")
	file2 := []byte("second file content")
	manifestID := uuid.New()
	peerID := uuid.New()

	paths := [][]string{
		{"dir", "file1.txt"},
		{"dir", "file2.txt"},
	}

	m, err := c.BuildManifest(
		manifestID,
		[][]byte{file1, file2},
		paths,
		"mydir",
		[]string{"http://tracker:8080"},
		"",
		peerID,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Info.Length != 0 {
		t.Errorf("Length: expected 0 for multi-file, got %d", m.Info.Length)
	}
	if len(m.Info.Files) != 2 {
		t.Fatalf("Files: expected 2, got %d", len(m.Info.Files))
	}
	if m.Info.Files[0].Len != int64(len(file1)) {
		t.Errorf("Files[0].Len: got %d, want %d", m.Info.Files[0].Len, len(file1))
	}
	if m.Info.Files[1].Len != int64(len(file2)) {
		t.Errorf("Files[1].Len: got %d, want %d", m.Info.Files[1].Len, len(file2))
	}
	if len(m.Info.Files[0].Path) != 2 || m.Info.Files[0].Path[1] != "file1.txt" {
		t.Errorf("Files[0].Path: got %v", m.Info.Files[0].Path)
	}
}

func TestBuildManifestPiecesMatchEncode(t *testing.T) {
	c := &Codec{}
	data := makeBytes(5000, 0x42)
	manifestID := uuid.New()
	peerID := uuid.New()

	m, err := c.BuildManifest(manifestID, [][]byte{data}, nil, "big.bin", nil, "", peerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHashes, expectedChunkLen, err := c.encode([][]byte{data})
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	if m.Info.PieceLength != expectedChunkLen {
		t.Errorf("PieceLength: got %d, want %d", m.Info.PieceLength, expectedChunkLen)
	}
	if len(m.Info.Pieces) != len(expectedHashes) {
		t.Fatalf("Pieces count: got %d, want %d", len(m.Info.Pieces), len(expectedHashes))
	}
	for i := range expectedHashes {
		if m.Info.Pieces[i] != expectedHashes[i] {
			t.Errorf("Pieces[%d]: got %x, want %x", i, m.Info.Pieces[i], expectedHashes[i])
		}
	}
}

func TestMarshalUnmarshalRoundtrip(t *testing.T) {
	c := &Codec{}
	data := []byte("roundtrip test data")
	manifestID := uuid.New()
	peerID := uuid.New()

	original, err := c.BuildManifest(
		manifestID,
		[][]byte{data},
		nil,
		"roundtrip.txt",
		[]string{"http://tracker1:8080", "http://tracker2:9090"},
		"some comment",
		peerID,
	)
	if err != nil {
		t.Fatalf("BuildManifest error: %v", err)
	}

	encoded, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	decoded, err := Unmarshal(encoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// проверяем поля
	if decoded.Info.Name != original.Info.Name {
		t.Errorf("Name: got %q, want %q", decoded.Info.Name, original.Info.Name)
	}
	if decoded.Info.Length != original.Info.Length {
		t.Errorf("Length: got %d, want %d", decoded.Info.Length, original.Info.Length)
	}
	if decoded.Info.PieceLength != original.Info.PieceLength {
		t.Errorf("PieceLength: got %d, want %d", decoded.Info.PieceLength, original.Info.PieceLength)
	}
	if decoded.Comment != original.Comment {
		t.Errorf("Comment: got %q, want %q", decoded.Comment, original.Comment)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID: got %v, want %v", decoded.ID, original.ID)
	}
	if decoded.CreatedBy != original.CreatedBy {
		t.Errorf("CreatedBy: got %v, want %v", decoded.CreatedBy, original.CreatedBy)
	}
	if decoded.CreationDate.Unix() != original.CreationDate.Unix() {
		t.Errorf("CreationDate: got %v, want %v", decoded.CreationDate, original.CreationDate)
	}
	if len(decoded.AnnounceList) != len(original.AnnounceList) {
		t.Fatalf("AnnounceList len: got %d, want %d", len(decoded.AnnounceList), len(original.AnnounceList))
	}
	for i := range original.AnnounceList {
		if decoded.AnnounceList[i] != original.AnnounceList[i] {
			t.Errorf("AnnounceList[%d]: got %q, want %q", i, decoded.AnnounceList[i], original.AnnounceList[i])
		}
	}
	if len(decoded.Info.Pieces) != len(original.Info.Pieces) {
		t.Fatalf("Pieces count: got %d, want %d", len(decoded.Info.Pieces), len(original.Info.Pieces))
	}
	for i := range original.Info.Pieces {
		if decoded.Info.Pieces[i] != original.Info.Pieces[i] {
			t.Errorf("Pieces[%d]: got %x, want %x", i, decoded.Info.Pieces[i], original.Info.Pieces[i])
		}
	}
}

func TestMarshalUnmarshalMultipleFiles(t *testing.T) {
	c := &Codec{}
	file1 := []byte("content of file one")
	file2 := []byte("content of file two")
	manifestID := uuid.New()
	peerID := uuid.New()

	original, err := c.BuildManifest(
		manifestID,
		[][]byte{file1, file2},
		[][]string{{"a", "file1.txt"}, {"a", "file2.txt"}},
		"mydir",
		[]string{"http://tracker:8080"},
		"",
		peerID,
	)
	if err != nil {
		t.Fatalf("BuildManifest error: %v", err)
	}

	encoded, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	decoded, err := Unmarshal(encoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded.Info.Files) != 2 {
		t.Fatalf("Files count: got %d, want 2", len(decoded.Info.Files))
	}
	if decoded.Info.Files[0].Len != int64(len(file1)) {
		t.Errorf("Files[0].Len: got %d, want %d", decoded.Info.Files[0].Len, len(file1))
	}
	if decoded.Info.Files[1].Len != int64(len(file2)) {
		t.Errorf("Files[1].Len: got %d, want %d", decoded.Info.Files[1].Len, len(file2))
	}
	if len(decoded.Info.Files[0].Path) != 2 || decoded.Info.Files[0].Path[1] != "file1.txt" {
		t.Errorf("Files[0].Path: got %v", decoded.Info.Files[0].Path)
	}
}

func TestUnmarshalInvalidData(t *testing.T) {
	cases := [][]byte{
		{},
		[]byte("not bencode"),
		[]byte("i42"),
		[]byte("5:hi"),
	}
	for _, data := range cases {
		_, err := Unmarshal(data)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", data)
		}
	}
}

// тесты Decode

func TestDecodeSingleFile(t *testing.T) {
	c := &Codec{}
	original := []byte("hello, world!")
	chunks := [][]byte{original}
	fileLengths := []int64{int64(len(original))}

	files, err := c.Decode(chunks, fileLengths)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if string(files[0]) != string(original) {
		t.Errorf("file content mismatch: got %q, want %q", files[0], original)
	}
}

func TestDecodeMultipleChunks(t *testing.T) {
	c := &Codec{}
	chunks := [][]byte{[]byte("ABCD"), []byte("EFGH"), []byte("I")}
	fileLengths := []int64{9}

	files, err := c.Decode(chunks, fileLengths)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if string(files[0]) != "ABCDEFGHI" {
		t.Errorf("got %q, want %q", files[0], "ABCDEFGHI")
	}
}

func TestDecodeMultipleFiles(t *testing.T) {
	c := &Codec{}
	chunks := [][]byte{[]byte("AB"), []byte("CD"), []byte("E")}
	fileLengths := []int64{3, 2}

	files, err := c.Decode(chunks, fileLengths)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if string(files[0]) != "ABC" {
		t.Errorf("file 0: got %q, want %q", files[0], "ABC")
	}
	if string(files[1]) != "DE" {
		t.Errorf("file 1: got %q, want %q", files[1], "DE")
	}
}

func TestDecodeFileSplitAcrossChunkBoundary(t *testing.T) {
	c := &Codec{}
	chunks := [][]byte{[]byte("ABCD"), []byte("EF")}
	fileLengths := []int64{3, 3}

	files, err := c.Decode(chunks, fileLengths)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if string(files[0]) != "ABC" {
		t.Errorf("file 0: got %q, want %q", files[0], "ABC")
	}
	if string(files[1]) != "DEF" {
		t.Errorf("file 1: got %q, want %q", files[1], "DEF")
	}
}

func TestDecodeRoundtrip(t *testing.T) {
	c := &Codec{}
	fileA := []byte("The quick brown fox")
	fileB := []byte("jumps over the lazy dog")
	files := [][]byte{fileA, fileB}

	manifest, err := c.BuildManifest(
		uuid.New(), files, nil, "test", nil, "", uuid.New(),
	)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	chunkLen := manifest.Info.PieceLength
	var sumLen int64
	for _, f := range files {
		sumLen += int64(len(f))
	}
	readByte := func(pos int64) byte {
		for _, f := range files {
			if pos < int64(len(f)) {
				return f[pos]
			}
			pos -= int64(len(f))
		}
		return 0
	}
	chunksAmount := (sumLen + chunkLen - 1) / chunkLen
	chunks := make([][]byte, chunksAmount)
	var globalPos int64
	for i := int64(0); i < chunksAmount; i++ {
		remaining := sumLen - globalPos
		curLen := chunkLen
		if remaining < chunkLen {
			curLen = remaining
		}
		chunk := make([]byte, curLen)
		for j := int64(0); j < curLen; j++ {
			chunk[j] = readByte(globalPos)
			globalPos++
		}
		chunks[i] = chunk
	}

	fileLengths := make([]int64, len(manifest.Info.Files))
	for i, fm := range manifest.Info.Files {
		fileLengths[i] = fm.Len
	}

	decoded, err := c.Decode(chunks, fileLengths)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 files, got %d", len(decoded))
	}
	if string(decoded[0]) != string(fileA) {
		t.Errorf("file A mismatch:\n  got  %q\n  want %q", decoded[0], fileA)
	}
	if string(decoded[1]) != string(fileB) {
		t.Errorf("file B mismatch:\n  got  %q\n  want %q", decoded[1], fileB)
	}
}

func TestDecodeNoChunks(t *testing.T) {
	c := &Codec{}
	_, err := c.Decode(nil, []int64{5})
	if err == nil {
		t.Error("expected error for nil chunks, got nil")
	}
	_, err = c.Decode([][]byte{}, []int64{5})
	if err == nil {
		t.Error("expected error for empty chunks, got nil")
	}
}

func TestDecodeNoFileLengths(t *testing.T) {
	c := &Codec{}
	_, err := c.Decode([][]byte{[]byte("data")}, nil)
	if err == nil {
		t.Error("expected error for nil fileLengths, got nil")
	}
}

func TestDecodeNegativeFileLength(t *testing.T) {
	c := &Codec{}
	_, err := c.Decode([][]byte{[]byte("data")}, []int64{-1})
	if err == nil {
		t.Error("expected error for negative file length, got nil")
	}
}

func TestDecodeFileLengthsExceedChunks(t *testing.T) {
	c := &Codec{}
	_, err := c.Decode([][]byte{[]byte("ABCD")}, []int64{10})
	if err == nil {
		t.Error("expected error when file lengths exceed chunk data, got nil")
	}
}
