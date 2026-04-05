package codec

import (
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Codec struct{}

type FileMeta struct {
	Len  int64    // размер файла в байтах
	Path []string // элементы пути от корневой директории
}

type Info struct {
	PieceLength int64             // размер одного чанка в байтах
	Pieces      [][sha1.Size]byte // SHA-1 хеши всех чанков
	Name        string            // рекомендуемое имя файла или папки
	Length      int64             // длина файла в байтах (только если файл один)
	Files       []FileMeta        // метаданные файлов (если файлов несколько)
}

type ManifestFile struct {
	Info         Info      // информация о файлах и чанках
	AnnounceList []string  // список URL трекеров
	CreationDate time.Time // время создания манифеста
	Comment      string    // произвольный комментарий
	CreatedBy    uuid.UUID // UUID пира, создавшего манифест
}

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

// сливаем файлы в поток байт, разбиваем на чанки, хешируем
func (c *Codec) encode(files [][]byte) ([][sha1.Size]byte, int64, error) {
	if len(files) == 0 {
		return nil, 0, fmt.Errorf("encode: no files provided")
	}

	var sumlen int64 = 0
	for _, file := range files {
		sumlen += int64(len(file))
	}
	if sumlen == 0 {
		return nil, 0, fmt.Errorf("encode: all files are empty")
	}

	chunkLen := calcChunkLen(sumlen)
	chunksAmount := (sumlen + chunkLen - 1) / chunkLen
	hashedChunks := make([][sha1.Size]byte, chunksAmount)

	// байт по глобальной позиции в общем потоке
	readByte := func(pos int64) byte {
		for _, file := range files {
			flen := int64(len(file))
			if pos < flen {
				return file[pos]
			}
			pos -= flen
		}
		return 0
	}

	var globalPos int64 = 0
	for chunkIdx := int64(0); chunkIdx < chunksAmount; chunkIdx++ {
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

	return hashedChunks, chunkLen, nil
}

func (c *Codec) BuildManifest(
	files [][]byte, 		// содержимое файлов в виде срезов байт
	filePaths [][]string, 	// пути к файлам. Если файл один, оставим пустым
	name string, 			// рекомендуемое имя файла или папки
	trackers []string, 		// список URL трекеров
	comment string, 		// произвольный комментарий
	createdBy uuid.UUID, 	// UUID пира, создающего манифест
) (ManifestFile, error) {
	hashes, chunkLen, err := c.encode(files)
	if err != nil {
		return ManifestFile{}, fmt.Errorf("BuildManifest: %w", err)
	}

	info := Info{
		PieceLength: chunkLen,
		Pieces:      hashes,
		Name:        name,
	}

	if len(files) == 1 {
		info.Length = int64(len(files[0]))
	} else {
		info.Files = make([]FileMeta, len(files))
		for i, file := range files {
			meta := FileMeta{Len: int64(len(file))}
			if filePaths != nil && i < len(filePaths) {
				meta.Path = filePaths[i]
			}
			info.Files[i] = meta
		}
	}

	return ManifestFile{
		Info:         info,
		AnnounceList: trackers,
		CreationDate: time.Now().UTC(),
		Comment:      comment,
		CreatedBy:    createdBy,
	}, nil
}
