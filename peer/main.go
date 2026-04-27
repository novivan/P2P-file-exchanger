package main

import (
	"context"
	"crypto/sha1"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/adrg/xdg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"peer/codec"
	"peer/config"
	"peer/netutil"
	"peer/p2p"
	"peer/tracker_client"
)

//go:embed ui/*
var uiFS embed.FS

type TorrentInfo struct {
	ManifestID uuid.UUID `json:"manifest_id"`
	Name       string    `json:"name"`
	Role       string    `json:"role"` // "seeder" | "leecher" | "done"
	FilePath   string    `json:"file_path"`
	TotalLen   int64     `json:"total_len"`
	ChunkLen   int64     `json:"chunk_len"`
	Chunks     int       `json:"chunks"`
}

type PeerServer struct {
	peerID          uuid.UUID
	trackerURL      string
	p2pAddr         string // внешний адрес, нужен для announce
	downloadDir     string
	downloadWorkers int

	tracker *tracker_client.Client
	store   *p2p.DiskChunkStore
	seeder  *p2p.Seeder
	codec   *codec.Codec

	mu       sync.RWMutex
	torrents map[uuid.UUID]*TorrentInfo
}

func NewPeerServer(trackerURL, p2pListenAddr, p2pExternalAddr, downloadDir string, downloadWorkers int) (*PeerServer, error) {
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("NewPeerServer: mkdir %q: %w", downloadDir, err)
	}

	store := p2p.NewDiskChunkStore()

	seeder, err := p2p.NewSeeder(p2pListenAddr, store)
	if err != nil {
		return nil, fmt.Errorf("NewPeerServer: start seeder: %w", err)
	}

	externalAddr := p2pExternalAddr
	if externalAddr == "" {
		externalAddr = seeder.Addr().String()
	}

	peerID := uuid.New()
	tracker := tracker_client.NewClient(trackerURL)

	if err := tracker.RegisterPeer(peerID, externalAddr); err != nil {
		seeder.Close()
		return nil, fmt.Errorf("NewPeerServer: register on tracker: %w", err)
	}

	ps := &PeerServer{
		peerID:          peerID,
		trackerURL:      trackerURL,
		p2pAddr:         externalAddr,
		downloadDir:     downloadDir,
		downloadWorkers: downloadWorkers,
		tracker:         tracker,
		store:           store,
		seeder:          seeder,
		codec:           &codec.Codec{},
		torrents:        make(map[uuid.UUID]*TorrentInfo),
	}

	go func() {
		if err := seeder.Serve(); err != nil {
			log.Printf("seeder stopped: %v", err)
		}
	}()

	return ps, nil
}

func (ps *PeerServer) Close() error {
	return ps.seeder.Close()
}

func (ps *PeerServer) AddSeed(filePath, name, description string) (*TorrentInfo, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("AddSeed: abs %q: %w", filePath, err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("AddSeed: read %q: %w", absPath, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("AddSeed: file %q is empty", absPath)
	}

	if name == "" {
		name = filepath.Base(absPath)
	}

	manifestID := uuid.New()
	manifest, err := ps.codec.BuildManifest(
		manifestID,
		[][]byte{data},
		nil,
		name,
		description,
		[]string{ps.trackerURL},
		"",
		ps.peerID,
	)
	if err != nil {
		return nil, fmt.Errorf("AddSeed: build manifest: %w", err)
	}

	raw, err := codec.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("AddSeed: marshal manifest: %w", err)
	}

	if err := ps.tracker.UploadManifest(manifestID, name, manifest.Description, raw); err != nil {
		return nil, fmt.Errorf("AddSeed: upload manifest: %w", err)
	}

	if err := ps.store.Register(manifestID, []string{absPath}, manifest.Info.PieceLength); err != nil {
		return nil, fmt.Errorf("AddSeed: register in store: %w", err)
	}

	if err := ps.tracker.Announce(manifestID, ps.peerID); err != nil {
		return nil, fmt.Errorf("AddSeed: announce: %w", err)
	}

	info := &TorrentInfo{
		ManifestID: manifestID,
		Name:       name,
		Role:       "seeder",
		FilePath:   absPath,
		TotalLen:   int64(len(data)),
		ChunkLen:   manifest.Info.PieceLength,
		Chunks:     len(manifest.Info.Pieces),
	}
	ps.mu.Lock()
	ps.torrents[manifestID] = info
	ps.mu.Unlock()

	return info, nil
}

func (ps *PeerServer) Download(manifestID uuid.UUID) (*TorrentInfo, error) {
	raw, err := ps.tracker.GetManifest(manifestID)
	if err != nil {
		return nil, fmt.Errorf("Download: get manifest: %w", err)
	}
	manifest, err := codec.Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("Download: unmarshal manifest: %w", err)
	}

	if len(manifest.Info.Files) > 0 {
		return nil, fmt.Errorf("Download: multi-file manifests not supported yet")
	}
	totalLen := manifest.Info.Length
	if totalLen <= 0 {
		return nil, fmt.Errorf("Download: manifest has zero length")
	}

	name := manifest.Info.Name
	if name == "" {
		name = manifestID.String()
	}
	savePath := filepath.Join(ps.downloadDir, name)

	peers, err := ps.tracker.GetSeeders(manifestID)
	if err != nil {
		return nil, fmt.Errorf("Download: get seeders: %w", err)
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("Download: no seeders for manifest %s", manifestID)
	}

	var candidates []string
	for _, p := range peers {
		if p.ID == ps.peerID {
			continue
		}
		candidates = append(candidates, p.Address)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("Download: no external seeders for manifest %s", manifestID)
	}

	info := &TorrentInfo{
		ManifestID: manifestID,
		Name:       name,
		Role:       "leecher",
		FilePath:   savePath,
		TotalLen:   totalLen,
		ChunkLen:   manifest.Info.PieceLength,
		Chunks:     len(manifest.Info.Pieces),
	}
	ps.mu.Lock()
	ps.torrents[manifestID] = info
	ps.mu.Unlock()

	f, err := os.OpenFile(savePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("Download: create %q: %w", savePath, err)
	}
	if err := f.Truncate(totalLen); err != nil {
		f.Close()
		return nil, fmt.Errorf("Download: truncate %q: %w", savePath, err)
	}

	if err := ps.fetchChunksParallel(manifestID, manifest, candidates, f); err != nil {
		f.Close()
		os.Remove(savePath)
		return nil, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return nil, fmt.Errorf("Download: sync: %w", err)
	}
	f.Close()

	if err := ps.store.Register(manifestID, []string{savePath}, manifest.Info.PieceLength); err != nil {
		return nil, fmt.Errorf("Download: register for seeding: %w", err)
	}
	if err := ps.tracker.Announce(manifestID, ps.peerID); err != nil {
		log.Printf("Download: announce after download failed: %v", err)
	}

	ps.mu.Lock()
	info.Role = "seeder"
	ps.mu.Unlock()

	return info, nil
}

type chunkResult struct {
	idx  uint32
	data []byte
	err  error
}

func (ps *PeerServer) fetchChunksParallel(manifestID uuid.UUID, manifest codec.ManifestFile, candidates []string, f *os.File) error {
	chunksAmount := uint32(len(manifest.Info.Pieces))
	if chunksAmount == 0 {
		return nil
	}

	workers := ps.downloadWorkers
	if workers <= 0 {
		workers = config.DefaultDownloadWorkers
	}
	if workers > int(chunksAmount) {
		workers = int(chunksAmount)
	}

	jobs := make(chan uint32, chunksAmount)
	results := make(chan chunkResult, workers)

	for idx := uint32(0); idx < chunksAmount; idx++ {
		jobs <- idx
	}
	close(jobs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case idx, ok := <-jobs:
					if !ok {
						return
					}
					data, err := fetchOneChunk(idx, manifestID, manifest.Info.Pieces[idx], candidates, workerID)
					select {
					case results <- chunkResult{idx: idx, data: data, err: err}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(w)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	received := uint32(0)
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Download: chunk %d: %w", r.idx, r.err)
			}
			cancel()
			continue
		}
		offset := int64(r.idx) * manifest.Info.PieceLength
		if _, werr := f.WriteAt(r.data, offset); werr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Download: write chunk %d: %w", r.idx, werr)
			}
			cancel()
			continue
		}
		received++
	}

	if firstErr != nil {
		return firstErr
	}
	if received != chunksAmount {
		return fmt.Errorf("Download: got %d/%d chunks", received, chunksAmount)
	}
	return nil
}

func fetchOneChunk(idx uint32, manifestID uuid.UUID, expectedHash [sha1.Size]byte, candidates []string, workerID int) ([]byte, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates")
	}
	var lastErr error
	for i := 0; i < len(candidates); i++ {
		addr := candidates[(workerID+i)%len(candidates)]
		data, err := p2p.RequestChunk(addr, manifestID, idx)
		if err != nil {
			lastErr = err
			continue
		}
		if sha1.Sum(data) != expectedHash {
			lastErr = fmt.Errorf("sha1 mismatch from %s", addr)
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

func (ps *PeerServer) ListTorrents() []TorrentInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]TorrentInfo, 0, len(ps.torrents))
	for _, t := range ps.torrents {
		out = append(out, *t)
	}
	return out
}

func (ps *PeerServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"peer_id":      ps.peerID,
		"p2p_addr":     ps.p2pAddr,
		"tracker":      ps.trackerURL,
		"download_dir": ps.downloadDir,
		"torrents":     len(ps.torrents),
		"time":         time.Now().UTC(),
	})
}

const (
	descriptionMinLen = 30
	descriptionMaxLen = 8000
)

func validateDescription(description string) error {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return fmt.Errorf("description is required")
	}
	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount < descriptionMinLen {
		return fmt.Errorf("description is too short: %d characters, minimum is %d", runeCount, descriptionMinLen)
	}
	if runeCount > descriptionMaxLen {
		return fmt.Errorf("description is too long: %d characters, maximum is %d", runeCount, descriptionMaxLen)
	}
	return nil
}

func (ps *PeerServer) handleSeed(c *gin.Context) {
	var body struct {
		FilePath    string `json:"file_path" binding:"required"`
		Name        string `json:"name"`
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateDescription(body.Description); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	info, err := ps.AddSeed(body.FilePath, body.Name, strings.TrimSpace(body.Description))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (ps *PeerServer) handleDownload(c *gin.Context) {
	var body struct {
		ManifestID string `json:"manifest_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := uuid.Parse(body.ManifestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid manifest_id: %v", err)})
		return
	}
	info, err := ps.Download(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (ps *PeerServer) handleList(c *gin.Context) {
	c.JSON(http.StatusOK, ps.ListTorrents())
}

func (ps *PeerServer) handleListRemote(c *gin.Context) {
	metas, err := ps.tracker.ListManifests()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metas)
}

func (ps *PeerServer) handleSearch(c *gin.Context) {
	var req tracker_client.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := ps.tracker.Search(req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	trackerURL := cfg.Tracker.URL

	downloadDir := xdg.UserDirs.Download

	localIP := netutil.MustGetLocalIP()

	p2pListen := net.JoinHostPort(localIP, fmt.Sprintf("%d", cfg.Server.P2PPort))
	p2pExternal := p2pListen
	apiAddr := net.JoinHostPort(localIP, fmt.Sprintf("%d", cfg.Server.APIPort))

	ps, err := NewPeerServer(trackerURL, p2pListen, p2pExternal, downloadDir, cfg.Download.WorkersOrDefault())
	if err != nil {
		log.Fatalf("peer: %v", err)
	}
	defer ps.Close()

	log.Printf("peer started: id=%s p2p=%s (external=%s) api=%s tracker=%s downloads=%s",
		ps.peerID, ps.p2pAddr, p2pExternal, apiAddr, trackerURL, downloadDir)

	router := gin.Default()
	router.GET("/health", ps.handleHealth)
	router.POST("/seed", ps.handleSeed)
	router.POST("/download", ps.handleDownload)
	router.GET("/torrents", ps.handleList)
	router.GET("/manifests", ps.handleListRemote)
	router.POST("/search", ps.handleSearch)

	indexHTML, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		log.Fatalf("peer ui: read index.html: %v", err)
	}
	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	if err := router.Run(apiAddr); err != nil {
		log.Fatalf("peer api: %v", err)
	}
}
