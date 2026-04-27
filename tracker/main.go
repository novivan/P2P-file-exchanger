package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tracker/config"
	"tracker/embedder"
	"tracker/llm"
	"tracker/rerank"
	"tracker/store"
)

type Server struct {
	store     store.TrackerStore
	embedder  embedder.Embedder
	generator llm.Generator
	searchCfg config.SearchConfig
}

func NewServer(s store.TrackerStore, e embedder.Embedder, g llm.Generator, searchCfg config.SearchConfig) *Server {
	return &Server{store: s, embedder: e, generator: g, searchCfg: searchCfg}
}

func (srv *Server) hello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "tracker is running"})
}

func (srv *Server) uploadManifest(c *gin.Context) {
	idStr := c.Query("id")
	name := c.Query("name")
	description := c.Query("description")

	if idStr == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and name query params are required"})
		return
	}
	if description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description query param is required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid manifest id: %v", err)})
		return
	}

	data, err := io.ReadAll(c.Request.Body)
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body is empty or unreadable"})
		return
	}

	embedding, err := srv.embedder.Embed(c.Request.Context(), description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to embed description: %v", err)})
		return
	}

	if err := srv.store.SaveManifest(id, name, description, embedding, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id.String(), "name": name})
}

func (srv *Server) getManifest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid manifest id"})
		return
	}

	data, err := srv.store.GetManifest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (srv *Server) listManifests(c *gin.Context) {
	metas, err := srv.store.ListManifests()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metas)
}

func (srv *Server) registerPeer(c *gin.Context) {
	var body struct {
		ID      string `json:"id"      binding:"required"`
		Address string `json:"address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	peerID, err := uuid.Parse(body.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid peer id: %v", err)})
		return
	}

	if err := srv.store.RegisterPeer(peerID, body.Address); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": peerID.String(), "address": body.Address})
}

func (srv *Server) announce(c *gin.Context) {
	var body struct {
		ManifestID string `json:"manifest_id" binding:"required"`
		PeerID     string `json:"peer_id"     binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manifestID, err := uuid.Parse(body.ManifestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid manifest_id: %v", err)})
		return
	}
	peerID, err := uuid.Parse(body.PeerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid peer_id: %v", err)})
		return
	}

	if err := srv.store.AnnounceSeeder(manifestID, peerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (srv *Server) getSeeders(c *gin.Context) {
	idStr := c.Param("manifestID")
	manifestID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid manifest id"})
		return
	}

	peers, err := srv.store.GetSeeders(manifestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, peers)
}

func (srv *Server) search(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
		TopK  int    `json:"top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	finalN := req.TopK
	if finalN <= 0 {
		finalN = srv.searchCfg.FinalN
	}
	if finalN <= 0 {
		finalN = 3
	}

	queryEmbedding, err := srv.embedder.Embed(c.Request.Context(), req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to embed query: %v", err)})
		return
	}

	candidateK := srv.searchCfg.CandidateK
	if candidateK <= 0 {
		candidateK = finalN
	}

	candidates, err := srv.store.SearchManifests(queryEmbedding, candidateK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rerankApplied := false
	if srv.searchCfg.RerankEnabled && srv.generator != nil && len(candidates) > 0 {
		reranked, rerr := rerank.Rerank(c.Request.Context(), srv.generator, req.Query, candidates)
		if rerr != nil {
			log.Printf("search: rerank failed, falling back to cosine: %v", rerr)
		} else {
			candidates = reranked
			rerankApplied = true
		}
	}

	var results []store.SearchResult
	if rerankApplied {
		results = rerank.ApplyHybridScore(candidates, srv.searchCfg.RerankAlpha, finalN)
	} else {
		results = candidates
		if len(results) > finalN {
			results = results[:finalN]
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func main() {
	fmt.Println("Starting tracker...")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Current config:")
	log.Printf("\tembedder:  url=%s model=%s", cfg.Embedder.OllamaURL, cfg.Embedder.Model)
	log.Printf("\tgenerator: url=%s model=%q", cfg.Generator.OllamaURL, cfg.Generator.Model)
	log.Printf("\tsearch:    candidate_k=%d final_n=%d alpha=%.2f rerank=%v rewrite=%v explain=%v\n",
		cfg.Search.CandidateK, cfg.Search.FinalN, cfg.Search.RerankAlpha,
		cfg.Search.RerankEnabled, cfg.Search.QueryRewriteEnabled, cfg.Search.ExplainEnabled)

	var s store.TrackerStore
	if cfg.Store.Path != "" {
		bs, err := store.NewBoltStore(cfg.Store.Path)
		if err != nil {
			log.Fatalf("Failed to open bolt store at %q: %v", cfg.Store.Path, err)
		}
		defer bs.Close()
		s = bs
		log.Printf("Using BoltStore at %s", cfg.Store.Path)
	} else {
		s = store.NewInMemoryStore()
		log.Println("Using InMemoryStore (data will be lost on restart)")
	}

	var e embedder.Embedder
	if cfg.Embedder.OllamaURL != "" && cfg.Embedder.Model != "" {
		e = embedder.NewOllamaEmbedder(cfg.Embedder.OllamaURL, cfg.Embedder.Model)
		log.Printf("Using Ollama embedder at %s (model=%s)", cfg.Embedder.OllamaURL, cfg.Embedder.Model)
	} else {
		e = &embedder.NoopEmbedder{}
		log.Println("Using EmptyEmbedder (search will not work)")
	}

	var gen llm.Generator
	if cfg.Generator.Model != "" && cfg.Generator.OllamaURL != "" {
		gen = llm.NewOllamaGenerator(cfg.Generator.OllamaURL, cfg.Generator.Model)
		log.Printf("Using Ollama generator at %s (model=%s)", cfg.Generator.OllamaURL, cfg.Generator.Model)
	} else {
		log.Println("Generator disabled (LLM features will be skipped)")
	}

	srv := NewServer(s, e, gen, cfg.Search)

	router := gin.Default()

	router.GET("/hello", srv.hello)

	router.POST("/manifest", srv.uploadManifest)
	router.GET("/manifest/:id", srv.getManifest)
	router.GET("/manifests", srv.listManifests)

	router.POST("/peer", srv.registerPeer)
	router.POST("/announce", srv.announce)
	router.GET("/peers/:manifestID", srv.getSeeders)
	router.POST("/search", srv.search)

	router.Run(":8080")
}
