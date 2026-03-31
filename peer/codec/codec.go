package codec

import (
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// в файлах будет 4 типа данных как в .torrent (классика):
// 1) массив байт (например, строки)
// 2) число
// 3) список
// 4) ассоциативный массив (словарь)

type Codec struct{}

type FileMeta struct {
	len  int64    // размер файла (все размеры в байтах)
	path []string // элементы пути, разделенные слешем от корневой директории
}

type Info struct {
	piece_length int64
	pieces       [][sha1.Size]byte // хеши чанков
	name         string            // рекомендуемое имя файла (или папки, если манифест создавался из папки)
	length       int64             // если файл один, то длина файла в байтах (иначе ничего)
	files        []FileMeta        // если файлов несколько
}

type ManifestFile struct {
	info          Info      // информация о файлах
	announce_list []string  // список url'ов трекеров (пока планируется только один)
	creation_date time.Time // timestamp создания
	comment       string    // любой комментарий
	created_by    uuid.UUID // id пира
}

func (c *Codec) Say_hi() error {
	fmt.Println("Codec says \"Hi!\"")
	return nil
}

// calcChunkLen вычисляет размер чанка: минимальная степень двойки такая,
// что чанков получается не более 1024 штук (как в BitTorrent — piece size >= sumlen/1024).
func calcChunkLen(sumlen int64) int64 {
	if sumlen == 0 {
		return 1
	}
	var chunkLen int64 = 1
	for chunkLen*1024 < sumlen {
		chunkLen *= 2
	}
	return chunkLen
}

// Encode принимает содержимое файлов в виде срезов байт,
// разбивает их на чанки фиксированного размера (файлы конкатенируются),
// хеширует каждый чанк через SHA-1 и возвращает срез хешей.
//
// Последний чанк может быть короче остальных — он хешируется по реальной длине,
// без нулевого паддинга.
func (c *Codec) Encode(files [][]byte) ([][sha1.Size]byte, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("encode: no files provided")
	}

	// считаем суммарный размер
	var sumlen int64 = 0
	for _, file := range files {
		sumlen += int64(len(file))
	}
	if sumlen == 0 {
		return nil, fmt.Errorf("encode: all files are empty")
	}

	chunkLen := calcChunkLen(sumlen)
	chunksAmount := (sumlen + chunkLen - 1) / chunkLen

	hashedChunks := make([][sha1.Size]byte, chunksAmount)

	// Итерируемся по всем байтам через виртуальный "поток":
	// globalPos — текущая позиция в конкатенированном потоке байт
	var globalPos int64 = 0

	// readByte возвращает байт по глобальной позиции в конкатенированном потоке
	readByte := func(pos int64) byte {
		for _, file := range files {
			flen := int64(len(file))
			if pos < flen {
				return file[pos]
			}
			pos -= flen
		}
		// не должно случиться, если pos < sumlen
		return 0
	}

	for chunkIdx := int64(0); chunkIdx < chunksAmount; chunkIdx++ {
		// размер текущего чанка (последний может быть меньше)
		remaining := sumlen - globalPos
		curChunkLen := chunkLen
		if remaining < chunkLen {
			curChunkLen = remaining
		}

		chunk := make([]byte, curChunkLen)
		for i := int64(0); i < curChunkLen; i++ {
			chunk[i] = readByte(globalPos)
			globalPos++
		}

		hashedChunks[chunkIdx] = sha1.Sum(chunk)
	}

	return hashedChunks, nil
}
