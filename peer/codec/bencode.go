package codec

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Bencode — простой текстово-бинарный формат как в торренте
// Все операции ведутся с []byte, а не string, чтобы корректно
// обрабатывать бинарные данные (SHA-1 хеши в pieces).

// ManifestFile -> Bencode.
func Marshal(m ManifestFile) ([]byte, error) {
	var buf bytes.Buffer

	// Основной словарь, который содердит в себе все
	buf.WriteByte('d')

	// список URL трекеров
	buf.Write(bencodeKey("announce-list"))
	buf.WriteByte('l')
	for _, url := range m.AnnounceList {
		buf.WriteByte('l')
		buf.Write(bencodeStr(url))
		buf.WriteByte('e')
	}
	buf.WriteByte('e')

	buf.Write(bencodeKey("comment"))
	buf.Write(bencodeStr(m.Comment))

	// UUID
	buf.Write(bencodeKey("created by"))
	buf.Write(bencodeStr(m.CreatedBy.String()))

	// unix timestamp
	buf.Write(bencodeKey("creation date"))
	buf.Write(bencodeInt(m.CreationDate.Unix()))

	buf.Write(bencodeKey("info"))
	infoBytes, err := marshalInfo(m.Info)
	if err != nil {
		return nil, fmt.Errorf("Marshal: %w", err)
	}
	buf.Write(infoBytes)

	buf.WriteByte('e')
	return buf.Bytes(), nil
}

// Info -> Bencode-словарь.
func marshalInfo(info Info) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('d')

	if len(info.Files) > 0 {
		buf.Write(bencodeKey("files"))
		buf.WriteByte('l')
		for _, fm := range info.Files {
			buf.WriteByte('d')
			buf.Write(bencodeKey("length"))
			buf.Write(bencodeInt(fm.Len))
			buf.Write(bencodeKey("path"))
			buf.WriteByte('l')
			for _, p := range fm.Path {
				buf.Write(bencodeStr(p))
			}
			buf.WriteByte('e')
			buf.WriteByte('e')
		}
		buf.WriteByte('e')
	}

	if len(info.Files) == 0 {
		buf.Write(bencodeKey("length"))
		buf.Write(bencodeInt(info.Length))
	}

	buf.Write(bencodeKey("name"))
	buf.Write(bencodeStr(info.Name))

	buf.Write(bencodeKey("piece length"))
	buf.Write(bencodeInt(info.PieceLength))

	piecesBytes := make([]byte, len(info.Pieces)*sha1.Size)
	for i, h := range info.Pieces {
		copy(piecesBytes[i*sha1.Size:], h[:])
	}
	buf.Write(bencodeKey("pieces"))
	buf.Write(bencodeBytes(piecesBytes))

	buf.WriteByte('e')
	return buf.Bytes(), nil
}

// Bencode-данные -> ManifestFile.
func Unmarshal(data []byte) (ManifestFile, error) {
	val, _, err := decodeValue(data, 0)
	if err != nil {
		return ManifestFile{}, fmt.Errorf("Unmarshal: %w", err)
	}

	dict, ok := val.(map[string]any)
	if !ok {
		return ManifestFile{}, fmt.Errorf("Unmarshal: top-level value is not a dict")
	}

	var m ManifestFile

	if al, ok := dict["announce-list"]; ok {
		if outerList, ok := al.([]any); ok {
			for _, inner := range outerList {
				if innerList, ok := inner.([]any); ok {
					for _, u := range innerList {
						if s, ok := u.([]byte); ok {
							m.AnnounceList = append(m.AnnounceList, string(s))
						}
					}
				}
			}
		}
	}

	if v, ok := dict["comment"]; ok {
		if b, ok := v.([]byte); ok {
			m.Comment = string(b)
		}
	}

	if v, ok := dict["created by"]; ok {
		if b, ok := v.([]byte); ok {
			id, err := uuid.Parse(string(b))
			if err != nil {
				return ManifestFile{}, fmt.Errorf("Unmarshal: invalid uuid %q: %w", string(b), err)
			}
			m.CreatedBy = id
		}
	}

	if v, ok := dict["creation date"]; ok {
		if ts, ok := v.(int64); ok {
			m.CreationDate = time.Unix(ts, 0).UTC()
		}
	}

	if v, ok := dict["info"]; ok {
		infoDict, ok := v.(map[string]any)
		if !ok {
			return ManifestFile{}, fmt.Errorf("Unmarshal: info is not a dict")
		}
		info, err := unmarshalInfo(infoDict)
		if err != nil {
			return ManifestFile{}, fmt.Errorf("Unmarshal: %w", err)
		}
		m.Info = info
	}

	return m, nil
}

// info (dict) -> Info (struct)
func unmarshalInfo(d map[string]any) (Info, error) {
	var info Info

	if v, ok := d["name"]; ok {
		if b, ok := v.([]byte); ok {
			info.Name = string(b)
		}
	}

	if v, ok := d["piece length"]; ok {
		info.PieceLength, _ = v.(int64)
	}

	if v, ok := d["pieces"]; ok {
		raw, ok := v.([]byte)
		if !ok {
			return Info{}, fmt.Errorf("unmarshalInfo: pieces is not a byte string")
		}
		if len(raw)%sha1.Size != 0 {
			return Info{}, fmt.Errorf("unmarshalInfo: pieces length %d is not a multiple of %d", len(raw), sha1.Size)
		}
		count := len(raw) / sha1.Size
		info.Pieces = make([][sha1.Size]byte, count)
		for i := range info.Pieces {
			copy(info.Pieces[i][:], raw[i*sha1.Size:(i+1)*sha1.Size])
		}
	}

	if v, ok := d["length"]; ok {
		info.Length, _ = v.(int64)
	}

	if v, ok := d["files"]; ok {
		list, ok := v.([]any)
		if !ok {
			return Info{}, fmt.Errorf("unmarshalInfo: files is not a list")
		}
		info.Files = make([]FileMeta, 0, len(list))
		for _, item := range list {
			fdict, ok := item.(map[string]any)
			if !ok {
				return Info{}, fmt.Errorf("unmarshalInfo: file entry is not a dict")
			}
			var fm FileMeta
			if lv, ok := fdict["length"]; ok {
				fm.Len, _ = lv.(int64)
			}
			if pv, ok := fdict["path"]; ok {
				if pathList, ok := pv.([]any); ok {
					for _, p := range pathList {
						if b, ok := p.([]byte); ok {
							fm.Path = append(fm.Path, string(b))
						}
					}
				}
			}
			info.Files = append(info.Files, fm)
		}
	}

	return info, nil
}

// декодируем одно значение начиная с позиции pos
func decodeValue(data []byte, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("decodeValue: unexpected end of data at pos %d", pos)
	}
	switch {
	case data[pos] == 'i':
		return decodeInt(data, pos)
	case data[pos] == 'l':
		return decodeList(data, pos)
	case data[pos] == 'd':
		return decodeDict(data, pos)
	case data[pos] >= '0' && data[pos] <= '9':
		return decodeByteString(data, pos)
	default:
		return nil, pos, fmt.Errorf("decodeValue: unknown type marker %q at pos %d", data[pos], pos)
	}
}

func decodeInt(data []byte, pos int) (int64, int, error) {
	end := pos + 1
	for end < len(data) && data[end] != 'e' {
		end++
	}
	if end >= len(data) {
		return 0, pos, fmt.Errorf("decodeInt: no closing 'e' found starting at pos %d", pos)
	}
	n, err := strconv.ParseInt(string(data[pos+1:end]), 10, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("decodeInt: %w", err)
	}
	return n, end + 1, nil
}

func decodeByteString(data []byte, pos int) ([]byte, int, error) {
	colon := pos
	for colon < len(data) && data[colon] != ':' {
		colon++
	}
	if colon >= len(data) {
		return nil, pos, fmt.Errorf("decodeByteString: no colon found starting at pos %d", pos)
	}
	length, err := strconv.Atoi(string(data[pos:colon]))
	if err != nil {
		return nil, pos, fmt.Errorf("decodeByteString: invalid length: %w", err)
	}
	start := colon + 1
	end := start + length
	if end > len(data) {
		return nil, pos, fmt.Errorf("decodeByteString: data too short: need %d bytes at pos %d, have %d", length, start, len(data)-start)
	}
	// копируем срез, чтоб не держать ссылку на весь буффер
	result := make([]byte, length)
	copy(result, data[start:end])
	return result, end, nil
}

func decodeList(data []byte, pos int) ([]any, int, error) {
	pos++ // 'l'
	var result []any
	for pos < len(data) && data[pos] != 'e' {
		val, next, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, err
		}
		result = append(result, val)
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("decodeList: no closing 'e' found")
	}
	return result, pos + 1, nil // 'e'
}

func decodeDict(data []byte, pos int) (map[string]any, int, error) {
	pos++ // 'd'
	result := make(map[string]any)
	for pos < len(data) && data[pos] != 'e' {
		keyBytes, next, err := decodeByteString(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("decodeDict key: %w", err)
		}
		key := string(keyBytes)
		pos = next
		val, next, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("decodeDict value for key %q: %w", key, err)
		}
		result[key] = val
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("decodeDict: no closing 'e' found")
	}
	return result, pos + 1, nil // 'e'
}

func bencodeKey(s string) []byte {
	return bencodeStr(s)
}

func bencodeStr(s string) []byte {
	prefix := strconv.Itoa(len(s)) + ":"
	result := make([]byte, len(prefix)+len(s))
	copy(result, prefix)
	copy(result[len(prefix):], s)
	return result
}

// кодируем байты как строку
func bencodeBytes(b []byte) []byte {
	prefix := strconv.Itoa(len(b)) + ":"
	result := make([]byte, len(prefix)+len(b))
	copy(result, prefix)
	copy(result[len(prefix):], b)
	return result
}

func bencodeInt(n int64) []byte {
	s := "i" + strconv.FormatInt(n, 10) + "e"
	return []byte(s)
}

func bencodeDict(m map[string][]byte) []byte {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('d')
	for _, k := range keys {
		buf.Write(bencodeKey(k))
		buf.Write(m[k])
	}
	buf.WriteByte('e')
	return buf.Bytes()
}
