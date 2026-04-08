package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/google/uuid"
)

// Протокол обмена чанками (бинарный, фиксированная длина запроса):
//
// Запрос (leecher → seeder), 20 байт:
//   0 - 16  — UUID манифеста (16 байт, big-endian)
//   16 - 20 — индекс чанка (uint32, big-endian)
//
// Ответ (seeder → leecher):
//   0 - 4   — длина данных (uint32, big-endian)
//   4 - 4+N — байты чанка

const requestSize = 20

type ChunkRequest struct {
	ManifestID uuid.UUID
	ChunkIndex uint32
}

type ChunkStore interface {
	GetChunk(manifestID uuid.UUID, chunkIndex uint32) ([]byte, error)
}

// Функциональность Seeder'а
type Seeder struct {
	store    ChunkStore
	listener net.Listener
}

func NewSeeder(addr string, store ChunkStore) (*Seeder, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("NewSeeder: %w", err)
	}
	return &Seeder{store: store, listener: ln}, nil
}

func (s *Seeder) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *Seeder) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Seeder) Close() error {
	return s.listener.Close()
}

func (s *Seeder) handleConn(conn net.Conn) {
	defer conn.Close()

	req, err := readRequest(conn)
	if err != nil {
		return
	}

	data, err := s.store.GetChunk(req.ManifestID, req.ChunkIndex)
	if err != nil {
		_ = writeUint32(conn, 0)
		return
	}

	if err := writeUint32(conn, uint32(len(data))); err != nil {
		return
	}
	_, _ = conn.Write(data)
}

// теперь функции leecher'а
func RequestChunk(addr string, manifestID uuid.UUID, chunkIndex uint32) ([]byte, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("RequestChunk: dial %s: %w", addr, err)
	}
	defer conn.Close()

	req := make([]byte, requestSize)
	copy(req[:16], manifestID[:])
	binary.BigEndian.PutUint32(req[16:], chunkIndex)

	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("RequestChunk: write request: %w", err)
	}

	length, err := readUint32(conn)
	if err != nil {
		return nil, fmt.Errorf("RequestChunk: read length: %w", err)
	}
	if length == 0 {
		return nil, fmt.Errorf("RequestChunk: seeder returned empty chunk (chunk not found or error)")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("RequestChunk: read data: %w", err)
	}

	return data, nil
}

// utils
func readRequest(r io.Reader) (ChunkRequest, error) {
	buf := make([]byte, requestSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ChunkRequest{}, fmt.Errorf("readRequest: %w", err)
	}

	var id uuid.UUID
	copy(id[:], buf[:16])
	index := binary.BigEndian.Uint32(buf[16:])

	return ChunkRequest{ManifestID: id, ChunkIndex: index}, nil
}

func writeUint32(w io.Writer, n uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, n)
	_, err := w.Write(buf)
	return err
}

func readUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}
