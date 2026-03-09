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
	piece_lengt int64
	pieces      []byte   // строка, содерджащая подряд все хеши чанков (мб потом передалю на string или на []rune)
	name        string   // рекомендуемое имя файла (или папки, если манифест создавался из папки)
	length      int64    //если файл один, то длина файла в байтах(инче ничего)
	files       FileMeta // если файлов несколько

}

type ManifestFile struct {
	info          Info      // информация о файлах
	announce_list []string  // список url'ов трекеров (пока планиурется только один)
	creation_date time.Time // timestamp создания
	comment       string    // любой комментарий
	created_by    uuid.UUID // id пира
}

func (c *Codec) Say_hi() error {
	fmt.Println("Codec says \"Hi!\"")
	return nil
}

func (c *Codec) Encode(files [][]byte) ([]byte, error) {
	files_amount := int64(len(files))
	var sumlen int64 = 0
	for _, file := range files {
		sumlen += int64(len(file))
	}
	var chank_len int64 = 1
	for chank_len*1024 < sumlen {
		chank_len *= 2
	}

	var chanks_amount int64 = (sumlen + chank_len - 1) / chank_len

	chanks := make([][]byte, chanks_amount)
	hashed_chanks := make([][sha1.Size]byte, chanks_amount)

	for i := range chanks {
		chanks[i] = make([]byte, chank_len)
	}

	var chank_idx int64 = 0
	var cur_shift_in_file int64 = 0
	// заполняем чанки
	for file_idx, file := range files {
		var file_len int64 = int64(len(file))
		for cur_shift_in_file < int64(len(file)) {
			if file_len-cur_shift_in_file < chank_len {
				copy(chanks[chank_idx], file[cur_shift_in_file:])
				if file_idx < int(files_amount)-1 {
					var filled int64 = file_len - cur_shift_in_file
					copy(chanks[chank_idx][filled:], files[file_idx+1][:chank_len-filled])
					cur_shift_in_file = chank_len - filled
				}
			} else {
				copy(chanks[chank_idx], file[cur_shift_in_file:cur_shift_in_file+chank_len])
				cur_shift_in_file += chank_len
			}

		}

	}

	for idx := range hashed_chanks {
		var err error
		hashed_chanks[idx], err = c.get_chunk_hash(chanks[idx])
		if err != nil {
			panic(err)
		}
	}

	// тут будем собирать уже наш файлик
	//пока так:
	return []byte{}, nil

}

func (c *Codec) get_chunk_hash(chank []byte) ([sha1.Size]byte, error) {
	ret := sha1.Sum(chank)
	return ret, nil
}

/*
// возвращать будет байты манифест файла
func (c *Codec) Encode(files []os.File) ([]byte, error) {
	sumlen := 0
	for file := range files {
		statistics := file.Stat()

		length := statistics.Size()

		sumlen += length
	}
	return make([]byte, sumlen), nil

}
*/
