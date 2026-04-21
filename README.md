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

## Внимание: Для использование ии-функций на трекере нужно предварительно установить модели:

1) скачать ollama
```
brew install ollama
# или
apt-get install ollama
```

2) Запустить Ollama:
```bash
ollama serve # слушает на порту 11434
```

3) Скачать модель для семантического поиска:
```bash
ollama pull bge-m3
```
это пока не реализовано:
```
ollama pull qwen2.5:1.5b
```

## Локальная демонстрация (одна машина)

### 1. Запустить трекер

**Без ИИ-поиска (NoopEmbedder):**
```bash
cd tracker
go run . # слушает :8080
```

**С ИИ-поиском (Ollama):**
```bash
cd tracker
OLLAMA_URL=http://localhost:11434 go run .
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
./peerctl -api http://127.0.0.1:9090 seed \
    --description "Короткая содержательная аннотация файла" \
    /path/to/file.bin
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

**Поиск манифестов по описанию:**
```bash
./peerctl -api http://127.0.0.1:9091 search "документы по программированию"
```

## API пира

| Метод | Путь | Описание |
|---|---|---|
| GET  | `/health`    | статус пира |
| POST | `/seed`      | `{file_path, description, name?}` → добавить в раздачу |
| POST | `/download`  | `{manifest_id}` → скачать |
| GET  | `/torrents`  | свои торренты |
| GET  | `/manifests` | список манифестов на трекере |
| POST | `/search`    | `{query, top_k}` → поиск по описанию |

## Переменные окружения

### Пира

| Переменная | Дефолт | Что задаёт |
|---|---|---|
| `TRACKER_URL`        | `http://localhost:8080` | адрес трекера |
| `P2P_ADDR`           | `127.0.0.1:0`           | на чём слушать TCP для раздачи |
| `P2P_EXTERNAL_ADDR`  | авто                     | адрес, который сообщаем трекеру |
| `DOWNLOAD_DIR`       | `./downloads`            | куда сохранять скачанное |
| `API_ADDR`           | `127.0.0.1:9090`         | адрес HTTP API |

### Трекера

| Переменная | Дефолт | Что задаёт |
|---|---|---|
| `OLLAMA_URL`         | не задан                 | адрес Ollama API (например, `http://localhost:11434`) |
