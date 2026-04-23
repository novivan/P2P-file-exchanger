package rerank

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"tracker/llm"
	"tracker/store"
)

const systemPrompt = `Ты помощник в системе поиска файлов. Оцени релевантность каждого кандидата запросу пользователя.
Верни строго JSON-массив вида [{"id":"<uuid>","score":<число от 0 до 10>}]. Без пояснений, без текста вне JSON.
Шкала: 0 — совсем не релевантно, 10 — идеально подходит. Учитывай смысл названия и описания файла.`

type scoreItem struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
}

func Rerank(ctx context.Context, gen llm.Generator, query string, candidates []store.SearchResult) ([]store.SearchResult, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	prompt := buildPrompt(query, candidates)

	raw, err := gen.Chat(ctx, systemPrompt, prompt, true)
	if err != nil {
		return nil, fmt.Errorf("rerank: llm chat: %w", err)
	}

	scores, err := parseScores(raw, candidates)
	if err != nil {
		return nil, fmt.Errorf("rerank: parse scores: %w", err)
	}

	out := make([]store.SearchResult, len(candidates))
	copy(out, candidates)
	for i := range out {
		if s, ok := scores[out[i].ID]; ok {
			out[i].LLMScore = s
		}
	}
	return out, nil
}

func ApplyHybridScore(results []store.SearchResult, alpha float32, finalN int) []store.SearchResult {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	out := make([]store.SearchResult, len(results))
	copy(out, results)
	for i := range out {
		out[i].Score = alpha*out[i].CosineScore + (1-alpha)*(out[i].LLMScore/10.0)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	if finalN > 0 && len(out) > finalN {
		out = out[:finalN]
	}
	return out
}

func buildPrompt(query string, candidates []store.SearchResult) string {
	var b strings.Builder
	b.WriteString("Запрос пользователя: ")
	b.WriteString(query)
	b.WriteString("\n\nКандидаты:\n")
	for _, c := range candidates {
		fmt.Fprintf(&b, "\tid: %s\n\tname: %s\n\tdescription: %s\n\n", c.ID.String(), c.Name, c.Description)
	}
	b.WriteString("\nВерни JSON-массив оценок в формате [{\"id\":\"<uuid>\",\"score\":<0..10>}] для всех указанных id.")
	return b.String()
}

func parseScores(raw string, candidates []store.SearchResult) (map[uuid.UUID]float32, error) {
	payload := extractJSONArray(raw)
	if payload == "" {
		return nil, fmt.Errorf("no JSON array in response: %q", raw)
	}

	var items []scoreItem
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		var wrapper struct {
			Scores []scoreItem `json:"scores"`
		}
		if err2 := json.Unmarshal([]byte(raw), &wrapper); err2 == nil && len(wrapper.Scores) > 0 {
			items = wrapper.Scores
		} else {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
	}

	known := make(map[uuid.UUID]struct{}, len(candidates))
	for _, c := range candidates {
		known[c.ID] = struct{}{}
	}

	result := make(map[uuid.UUID]float32, len(items))
	for _, it := range items {
		id, err := uuid.Parse(it.ID)
		if err != nil {
			continue
		}
		if _, ok := known[id]; !ok {
			continue
		}
		score := it.Score
		if score < 0 {
			score = 0
		}
		if score > 10 {
			score = 10
		}
		result[id] = score
	}
	return result, nil
}

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}
