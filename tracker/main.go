package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tracker/store"
)

type Server struct {
	store store.TrackerStore
}

func NewServer(s store.TrackerStore) *Server {
	return &Server{store: s}
}

func (srv *Server) hello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "tracker is running"})
}

func (srv *Server) uploadManifest(c *gin.Context) {
	idStr := c.Query("id")
	name := c.Query("name")

	if idStr == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and name query params are required"})
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

	if err := srv.store.SaveManifest(id, name, data); err != nil {
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

func main() {
	fmt.Println("Starting tracker...")

	s := store.NewInMemoryStore()
	srv := NewServer(s)

	router := gin.Default()

	router.GET("/hello", srv.hello)

	router.POST("/manifest", srv.uploadManifest)
	router.GET("/manifest/:id", srv.getManifest)
	router.GET("/manifests", srv.listManifests)

	router.POST("/peer", srv.registerPeer)
	router.POST("/announce", srv.announce)
	router.GET("/peers/:manifestID", srv.getSeeders)

	router.Run(":8080")
}
