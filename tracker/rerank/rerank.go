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

const systemPrompt = `You are a relevance ranker for a P2P file search system.

TASK: For EACH candidate file below, output an integer score from 0 to 10 reflecting how well its name and description match the user's query.

SCORING RUBRIC:
- 0-2: completely unrelated to the query
- 3-4: tangentially related, shares only a topic keyword
- 5-6: partially matches — covers some aspects of the query
- 7-8: strongly matches — covers the main intent of the query
- 9-10: perfect match — exactly what the user is looking for

OUTPUT FORMAT (STRICT):
You MUST return a JSON array with one object per candidate, in the SAME ORDER as given.
Each object MUST have exactly two keys: "id" (string, the candidate's UUID) and "score" (integer 0..10).
Return ONLY the JSON array. No prose, no explanation, no markdown code fences.
Even if there is only one candidate, you MUST return an array with one element.

EXAMPLE (for 2 candidates and query "рецепт борща"):
[
  {"id": "11111111-1111-1111-1111-111111111111", "score": 9},
  {"id": "22222222-2222-2222-2222-222222222222", "score": 2}
]

Now read the user's query and candidates, then produce the JSON array.`

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
	fmt.Fprintf(&b, "User query: %s\n\n", query)
	fmt.Fprintf(&b, "Candidates (%d total):\n\n", len(candidates))
	for i, c := range candidates {
		fmt.Fprintf(&b, "#%d\n  id: %s\n  name: %s\n  description: %s\n\n", i+1, c.ID.String(), c.Name, c.Description)
	}
	fmt.Fprintf(&b, "Return a JSON array with EXACTLY %d objects (one per candidate, same order), each of shape {\"id\":\"<uuid>\",\"score\":<integer 0..10>}. No prose, no markdown.", len(candidates))
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
