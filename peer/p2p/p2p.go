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
//   [0:16]  — UUID манифеста (16 байт, big-endian)
//   [16:20] — индекс чанка (uint32, big-endian)
//
// Ответ (seeder → leecher):
//   [0:4]   — длина данных (uint32, big-endian)
//   [4:4+N] — байты чанка

const requestSize = 20 // 16 байт UUID + 4 байта индекс

// ChunkRequest — распарсенный запрос от leecher'а
type ChunkRequest struct {
	ManifestID uuid.UUID
	ChunkIndex uint32
}

// ChunkStore — интерфейс хранилища чанков на стороне seeder'а.
// Возвращает байты чанка по UUID манифеста и индексу.
type ChunkStore interface {
	GetChunk(manifestID uuid.UUID, chunkIndex uint32) ([]byte, error)
}

// ---- Seeder (TCP-сервер) ---------------------------------------------------

// Seeder принимает входящие TCP-соединения и отдаёт чанки по запросу.
type Seeder struct {
	store    ChunkStore
	listener net.Listener
}

// NewSeeder создаёт Seeder, слушающий на указанном адресе (например, ":9000").
func NewSeeder(addr string, store ChunkStore) (*Seeder, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("NewSeeder: %w", err)
	}
	return &Seeder{store: store, listener: ln}, nil
}

// Addr возвращает адрес, на котором слушает seeder (полезно когда порт = 0).
func (s *Seeder) Addr() net.Addr {
	return s.listener.Addr()
}

// Serve запускает цикл приёма соединений. Блокирует до вызова Close().
func (s *Seeder) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// listener закрыт — нормальное завершение
			return err
		}
		go s.handleConn(conn)
	}
}

// Close останавливает seeder.
func (s *Seeder) Close() error {
	return s.listener.Close()
}

// handleConn обрабатывает одно соединение: читает запрос, отдаёт чанк.
func (s *Seeder) handleConn(conn net.Conn) {
	defer conn.Close()

	req, err := readRequest(conn)
	if err != nil {
		// клиент отключился или прислал мусор — просто закрываем
		return
	}

	data, err := s.store.GetChunk(req.ManifestID, req.ChunkIndex)
	if err != nil {
		// отправляем ответ с нулевой длиной как сигнал ошибки
		_ = writeUint32(conn, 0)
		return
	}

	if err := writeUint32(conn, uint32(len(data))); err != nil {
		return
	}
	_, _ = conn.Write(data)
}

// ---- Leecher (TCP-клиент) --------------------------------------------------

// RequestChunk подключается к seeder'у по адресу addr, запрашивает чанк
// с индексом chunkIndex из манифеста manifestID и возвращает его байты.
func RequestChunk(addr string, manifestID uuid.UUID, chunkIndex uint32) ([]byte, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("RequestChunk: dial %s: %w", addr, err)
	}
	defer conn.Close()

	// отправляем запрос: 16 байт UUID + 4 байта индекс
	req := make([]byte, requestSize)
	copy(req[:16], manifestID[:])
	binary.BigEndian.PutUint32(req[16:], chunkIndex)

	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("RequestChunk: write request: %w", err)
	}

	// читаем длину ответа
	length, err := readUint32(conn)
	if err != nil {
		return nil, fmt.Errorf("RequestChunk: read length: %w", err)
	}
	if length == 0 {
		return nil, fmt.Errorf("RequestChunk: seeder returned empty chunk (chunk not found or error)")
	}

	// читаем данные чанка
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("RequestChunk: read data: %w", err)
	}

	return data, nil
}

// ---- вспомогательные функции -----------------------------------------------

// readRequest читает 20-байтный запрос из соединения.
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

// writeUint32 записывает uint32 в big-endian в writer.
func writeUint32(w io.Writer, n uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, n)
	_, err := w.Write(buf)
	return err
}

// readUint32 читает uint32 в big-endian из reader.
func readUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}
