package codec

import (
	"fmt"
)

// в файлах будет 4 типа данных как в .torrent (классика):
// 1) массив байт (например, строки)
// 2) число
// 3) список
// 4) ассоциативный массив (словарь)

type Codec struct{}

func (c *Codec) Say_hi() error {
	fmt.Println("Codec says \"Hi!\"")
	return nil
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
