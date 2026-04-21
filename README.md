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

### 1. Настроить конфигурацию

Отредактируйте файлы конфигурации перед запуском:

- [`tracker/config.yaml`](tracker/config.yaml) — настройки трекера (Ollama URL)
- [`peer/config.yaml`](peer/config.yaml) — настройки пира (адрес трекера, порты)

### 2. Запустить трекер

```bash
cd tracker
go run . # слушает :8080
```

### 3. Запустить пир-«сидер»
В другом терминале:
```bash
cd peer
go run .
```


### 4. Запустить второй пир (опционально)
В третьем терминале:
```bash
cd peer
go run .
```

### 5. Пользоваться через CLI
Собрать `peerctl`:
```bash
cd peer
go build -o peerctl ./cmd/peerctl
```

На первом пире начать раздачу файла:
```bash
./peerctl seed \
    --description "Короткая содержательная аннотация файла" \
    /path/to/file.bin
# вернётся JSON с manifest_id
```

Посмотреть манифесты на трекере:
```bash
./peerctl manifests
```

На втором пире скачать файл по `manifest_id`:
```bash
./peerctl download <manifest_id>
```

Посмотреть свои торренты:
```bash
./peerctl list
```


**Поиск манифестов по описанию:**
```bash
./peerctl search "документы по программированию"
```
