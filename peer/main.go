package main

import (
	"fmt"
	"peer/codec"
)

func main() {
	fmt.Println("Запускаем клиент(пир)...")
	codec := codec.Codec{}
	codec.Say_hi()
}
