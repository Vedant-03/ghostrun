package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"ghostrun/internal/callgraph"
	"ghostrun/internal/loader"
	"ghostrun/internal/trace"
)

//go:embed ui/*
var uiFS embed.FS

// Server serves the DryRun web UI and API
type Server struct {
	port      int
	trace     *trace.TraceNode
	callgraph *callgraph.Graph
	pkgData   *loader.PackageData
}

// New creates a new server
func New(port int) *Server {
	return &Server{port: port}
}

// SetTrace sets the trace data to serve
func (s *Server) SetTrace(t *trace.TraceNode) {
	s.trace = t
}

// SetCallGraph sets the call graph data to serve
func (s *Server) SetCallGraph(g *callgraph.Graph) {
	s.callgraph = g
}

// SetPackageData sets the package data for source code serving
func (s *Server) SetPackageData(pd *loader.PackageData) {
	s.pkgData = pd
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/trace", s.handleTrace)
	mux.HandleFunc("/api/callgraph", s.handleCallGraph)
	mux.HandleFunc("/api/source", s.handleSource)
	mux.HandleFunc("/api/info", s.handleInfo)

	// Serve embedded UI files
	uiContent, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return fmt.Errorf("embedding UI: %w", err)
	}
	fileServer := http.FileServer(http.FS(uiContent))
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.trace == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "no trace data"})
		return
	}

	json.NewEncoder(w).Encode(s.trace)
}

func (s *Server) handleCallGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.callgraph == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "no call graph data"})
		return
	}

	json.NewEncoder(w).Encode(s.callgraph)
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	file := r.URL.Query().Get("file")
	if file == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "file parameter required"})
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	lines := strings.Split(string(data), "\n")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":  file,
		"lines": lines,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	info := map[string]interface{}{
		"has_trace":     s.trace != nil,
		"has_callgraph": s.callgraph != nil,
	}

	if s.trace != nil {
		info["mode"] = "trace"
		info["func_name"] = s.trace.FuncName
	} else if s.callgraph != nil {
		info["mode"] = "navigate"
		info["nodes"] = len(s.callgraph.Nodes)
		info["edges"] = len(s.callgraph.Edges)
	}

	if s.pkgData != nil {
		info["functions"] = s.pkgData.ListFunctions()
	}

	json.NewEncoder(w).Encode(info)
}
