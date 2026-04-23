package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"peer/config"
	"peer/netutil"
)

const defaultAPI = "http://127.0.0.1:9090"

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	localIP := netutil.MustGetLocalIP()
	defaultAPIWithIP := fmt.Sprintf("http://%s:%d", localIP, cfg.Server.APIPort)

	apiFlag := flag.String("api", envOr("PEER_API", defaultAPIWithIP), "peer HTTP API base URL (or $PEER_API)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `peerctl — CLI для сервиса-пира (через его HTTP API)

Usage:
  peerctl [-api URL] <command> [args]

Commands:
  seed --description "TEXT" <file_path> [name]	попросить пира раздавать файл
  download <manifest_id>                       	попросить пира скачать манифест
  list                                         	торренты этого пира
  manifests                                    	список манифестов на трекере
  search "users_query"                          поиск манифестов по описанию (топ-3)
  health                                       	статус пира

Env:
  PEER_API                  базовый URL API пира (default %s)
`, defaultAPI)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	api, err := normalizeBase(*apiFlag)
	if err != nil {
		fatal("invalid -api: %v", err)
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "seed":
		cmdSeed(api, rest)
	case "download":
		cmdDownload(api, rest)
	case "list", "ls":
		cmdList(api)
	case "manifests":
		cmdManifests(api)
	case "search":
		cmdSearch(api, rest)
	case "health":
		cmdHealth(api)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", cmd)
		flag.Usage()
		os.Exit(2)
	}
}

func cmdSeed(api string, args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	description := fs.String("description", "", "описание файла (обязательно, будет использоваться для семантического поиска)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `seed: usage: peerctl seed --description "TEXT" <file_path> [name]

Flags:
  --description "TEXT"   описание файла (обязательно)
`)
	}
	if err := fs.Parse(args); err != nil {
		fatal("seed: parse flags: %v", err)
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fs.Usage()
		os.Exit(2)
	}
	desc := strings.TrimSpace(*description)
	if desc == "" {
		fatal("seed: --description обязателен и не может быть пустым")
	}

	absPath, err := filepath.Abs(rest[0])
	if err != nil {
		fatal("seed: abs path: %v", err)
	}
	body := map[string]string{
		"file_path":   absPath,
		"description": desc,
	}
	if len(rest) >= 2 {
		body["name"] = rest[1]
	}

	var resp map[string]any
	if err := postJSON(api+"/seed", body, &resp); err != nil {
		fatal("seed: %v", err)
	}
	printJSON(resp)
}

func cmdDownload(api string, args []string) {
	if len(args) < 1 {
		fatal("download: usage: peerctl download <manifest_id>")
	}
	body := map[string]string{"manifest_id": args[0]}

	var resp map[string]any
	if err := postJSON(api+"/download", body, &resp); err != nil {
		fatal("download: %v", err)
	}
	printJSON(resp)
}

func cmdList(api string) {
	var resp []map[string]any
	if err := getJSON(api+"/torrents", &resp); err != nil {
		fatal("list: %v", err)
	}
	if len(resp) == 0 {
		fmt.Println("(нет торрентов)")
		return
	}
	fmt.Printf("%-38s  %-8s  %-6s  %-12s  %s\n", "MANIFEST_ID", "ROLE", "CHUNKS", "SIZE", "NAME")
	for _, t := range resp {
		fmt.Printf("%-38v  %-8v  %-6v  %-12v  %v\n",
			t["manifest_id"], t["role"], t["chunks"], t["total_len"], t["name"])
	}
}

func cmdManifests(api string) {
	var resp []map[string]any
	if err := getJSON(api+"/manifests", &resp); err != nil {
		fatal("manifests: %v", err)
	}
	if len(resp) == 0 {
		fmt.Println("(трекер пуст)")
		return
	}
	for i, m := range resp {
		if i > 0 {
			fmt.Println()
			fmt.Println()
		}
		fmt.Printf("ID:          %v\n", m["ID"])
		fmt.Printf("\tNAME:        %v\n", m["Name"])
		fmt.Printf("\tCREATED_AT:  %v\n", m["CreatedAt"])
		fmt.Printf("\tDESCRIPTION: %v\n", m["Description"])
	}
}

func cmdSearch(api string, args []string) {
	if len(args) < 1 {
		fatal("search: usage: peerctl search <query>")
	}
	query := strings.Join(args, " ")

	body := map[string]any{
		"query": query,
		"top_k": 3,
	}

	var resp struct {
		Results []struct {
			ID          string  `json:"ID"`
			Name        string  `json:"Name"`
			Description string  `json:"Description"`
			Score       float32 `json:"Score"`
			CosineScore float32 `json:"CosineScore"`
			LLMScore    float32 `json:"LLMScore"`
			Explanation string  `json:"Explanation"`
		} `json:"results"`
	}
	if err := postJSON(api+"/search", body, &resp); err != nil {
		fatal("search: %v", err)
	}

	if len(resp.Results) == 0 {
		fmt.Println("(ничего не найдено)")
		return
	}

	fmt.Printf("%-38s  %-30s  %-8s  %-8s  %-8s\n", "ID", "NAME", "SCORE", "COSINE", "LLM")
	for _, r := range resp.Results {
		fmt.Printf("\t%-38v  %-30v  %-8.4f  %-8.4f  %-8.2f\n", r.ID, r.Name, r.Score, r.CosineScore, r.LLMScore)
		if r.Description != "" {
			fmt.Printf(" \n\tDescription: %s\n", r.Description)
		}
		if r.Explanation != "" {
			fmt.Printf("\n\tExplanation: %s\n", r.Explanation)
		}
		fmt.Println()
	}
}

func cmdHealth(api string) {
	var resp map[string]any
	if err := getJSON(api+"/health", &resp); err != nil {
		fatal("health: %v", err)
	}
	printJSON(resp)
}

var httpClient = &http.Client{Timeout: 5 * time.Minute}

func postJSON(u string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := httpClient.Post(u, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOrError(resp, out)
}

func getJSON(u string, out any) error {
	resp, err := httpClient.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOrError(resp, out)
}

func decodeOrError(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, bytes.TrimSpace(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func normalizeBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("expected scheme://host[:port], got %q", raw)
	}
	s := u.Scheme + "://" + u.Host + u.Path
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "peerctl: "+format+"\n", args...)
	os.Exit(1)
}
