# P2P-file-exchanger

Курсовая работа 3 курса — P2P-система обмена файлами на Go, вдохновлённая BitTorrent.

Состоит из двух Go-модулей:
- [`tracker/`](tracker) — HTTP-трекер (каталог манифестов и списки сидеров);
- [`peer/`](peer) — сервис-пир (seeder + leecher), плюс CLI-клиент [`peerctl`](peer/cmd/peerctl) к этому сервису.

## Запуск тестов

В директории нужного модуля:
```bash
go test ./... -v
```

## Локальная демонстрация (одна машина)

### 1. Запустить трекер
```bash
cd tracker
go run . # слушает :8080
```

### 2. Запустить пир-«сидер»
В другом терминале:
```bash
cd peer
TRACKER_URL=http://localhost:8080 \
P2P_ADDR=127.0.0.1:7001 \
DOWNLOAD_DIR=./downloads-A \
API_ADDR=127.0.0.1:9090 \
go run .
```

### 3. Запустить пир-«личер»
В третьем терминале:
```bash
cd peer
TRACKER_URL=http://localhost:8080 \
P2P_ADDR=127.0.0.1:7002 \
DOWNLOAD_DIR=./downloads-B \
API_ADDR=127.0.0.1:9091 \
go run .
```

### 4. Пользоваться через CLI
Собрать `peerctl`:
```bash
cd peer
go build -o peerctl ./cmd/peerctl
```

На первом пире начать раздачу файла:
```bash
./peerctl -api http://127.0.0.1:9090 seed /path/to/file.bin
# вернётся JSON с manifest_id
```

Посмотреть манифесты на трекере:
```bash
./peerctl -api http://127.0.0.1:9091 manifests
```

На втором пире скачать файл по `manifest_id`:
```bash
./peerctl -api http://127.0.0.1:9091 download <manifest_id>
```

Посмотреть свои торренты:
```bash
./peerctl -api http://127.0.0.1:9091 list
```

Скачанный файл появится в `downloads-B/<name>` и автоматически начнёт раздаваться дальше.

## API пира

| Метод | Путь | Описание |
|---|---|---|
| GET  | `/health`    | статус пира |
| POST | `/seed`      | `{file_path, name?}` → добавить в раздачу |
| POST | `/download`  | `{manifest_id}` → скачать |
| GET  | `/torrents`  | свои торренты |
| GET  | `/manifests` | список манифестов на трекере |

## Переменные окружения пира

| Переменная | Дефолт | Что задаёт |
|---|---|---|
| `TRACKER_URL`        | `http://localhost:8080` | адрес трекера |
| `P2P_ADDR`           | `127.0.0.1:0`           | на чём слушать TCP для раздачи |
| `P2P_EXTERNAL_ADDR`  | авто                     | адрес, который сообщаем трекеру |
| `DOWNLOAD_DIR`       | `./downloads`            | куда сохранять скачанное |
| `API_ADDR`           | `127.0.0.1:9090`         | адрес HTTP API |
