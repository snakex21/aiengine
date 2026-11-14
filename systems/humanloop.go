// systems/humanloop.go — Human‑in‑the‑loop bridge for AI agents.
// Runs a localhost HTTP server so the agent can ask questions,
// display them in the game console, and wait for human input.
//
//   POST /ask     {"question":"...","options":["A","B"]}  → {"id":"..."}
//   GET  /pending                                         → {"id":"...","question":"..."} or {}
//   POST /answer  {"id":"...","answer":"A"}               → {"ok":true}

package systems

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PendingQuestion holds a question waiting for human input.
type PendingQuestion struct {
	ID        string   `json:"id"`
	Question  string   `json:"question"`
	Options   []string `json:"options"`
	Answer    chan string
	CreatedAt time.Time
}

// HumanLoop manages the HTTP server and pending questions.
type HumanLoop struct {
	mu      sync.Mutex
	server  *http.Server
	port    int
	pending *PendingQuestion
	active  bool
	// OnAsk is called when a question arrives. It runs on the HTTP server's
	// goroutine: hosts must hand the work over to their own thread (the
	// game queues it for the main loop).
	OnAsk    func(question string, options []string)
	shutdown chan struct{}
}

// NewHumanLoop creates a new human‑in‑the‑loop bridge.
func NewHumanLoop() *HumanLoop {
	return &HumanLoop{
		shutdown: make(chan struct{}),
	}
}

// Start begins listening on localhost:port.
func (hl *HumanLoop) Start(port int) error {
	hl.port = port

	mux := http.NewServeMux()
	mux.HandleFunc("/ask", hl.handleAsk)
	mux.HandleFunc("/pending", hl.handlePending)
	mux.HandleFunc("/answer", hl.handleAnswer)
	mux.HandleFunc("/", hl.handleStatus)

	hl.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	ln, err := net.Listen("tcp", hl.server.Addr)
	if err != nil {
		return fmt.Errorf("humanloop listen: %w", err)
	}
	hl.server.Addr = ln.Addr().String() // real port when port was 0

	hl.mu.Lock()
	hl.active = true
	hl.mu.Unlock()
	go func() {
		log.Printf("[HumanLoop] listening on http://%s", hl.server.Addr)
		if err := hl.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[HumanLoop] server error: %v", err)
		}
		hl.mu.Lock()
		hl.active = false
		hl.mu.Unlock()
	}()

	return nil
}

// Addr returns the server's listen address (host:port) after Start.
func (hl *HumanLoop) Addr() string {
	if hl.server == nil {
		return ""
	}
	return hl.server.Addr
}

// Active reports whether the HTTP server is running.
func (hl *HumanLoop) Active() bool {
	hl.mu.Lock()
	defer hl.mu.Unlock()
	return hl.active
}

// Stop gracefully shuts down the HTTP server.
func (hl *HumanLoop) Stop() {
	if hl.server != nil {
		hl.server.Close()
	}
	close(hl.shutdown)
}

// Ask posts a question and blocks until the human answers (or timeout).
// Returns the answer string.  Called by the game engine when agent sends a request.
func (hl *HumanLoop) Ask(question string, options []string) PendingQuestion {
	pq := PendingQuestion{
		ID:        fmt.Sprintf("q%d", time.Now().UnixNano()),
		Question:  question,
		Options:   options,
		Answer:    make(chan string, 1),
		CreatedAt: time.Now(),
	}

	hl.mu.Lock()
	hl.pending = &pq
	hl.mu.Unlock()

	if hl.OnAsk != nil {
		hl.OnAsk(question, options)
	}

	return pq
}

// Answer resolves the pending question with the given answer.
func (hl *HumanLoop) Answer(answer string) {
	hl.mu.Lock()
	pq := hl.pending
	hl.pending = nil
	hl.mu.Unlock()

	if pq != nil {
		select {
		case pq.Answer <- answer:
		default:
		}
	}
}

// Pending returns the current pending question, or nil.
func (hl *HumanLoop) Pending() *PendingQuestion {
	hl.mu.Lock()
	defer hl.mu.Unlock()
	return hl.pending
}

// --- HTTP handlers ---

func (hl *HumanLoop) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	pq := hl.Pending()
	if pq != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "waiting",
			"id":       pq.ID,
			"question": pq.Question,
			"options":  pq.Options,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "idle"})
	}
}

func (hl *HumanLoop) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Question == "" {
		http.Error(w, "question is required", http.StatusBadRequest)
		return
	}

	pq := hl.Ask(req.Question, req.Options)

	log.Printf("[HumanLoop] agent asks: %s  options: %v", pq.Question, pq.Options)

	// Wait for answer with timeout.
	select {
	case answer := <-pq.Answer:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"answer": answer})
	case <-time.After(120 * time.Second):
		hl.mu.Lock()
		if hl.pending != nil && hl.pending.ID == pq.ID {
			hl.pending = nil
		}
		hl.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"answer": "", "error": "timeout"})
	}
}

func (hl *HumanLoop) handlePending(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	pq := hl.Pending()
	if pq != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       pq.ID,
			"question": pq.Question,
			"options":  pq.Options,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}
}

func (hl *HumanLoop) handleAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	hl.mu.Lock()
	pq := hl.pending
	if pq != nil && pq.ID == req.ID {
		hl.pending = nil
		hl.mu.Unlock()
		pq.Answer <- req.Answer
		log.Printf("[HumanLoop] answer: %s → %s", req.ID, req.Answer)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
	hl.mu.Unlock()

	http.Error(w, "no matching pending question", http.StatusNotFound)
}

// --- helpers for console integration ---

// AnswerFromConsole tries to match console input to a pending question's options.
// Returns true if the input was consumed as a human‑loop answer.
func (hl *HumanLoop) AnswerFromConsole(input string) bool {
	hl.mu.Lock()
	pq := hl.pending
	hl.mu.Unlock()

	if pq == nil {
		return false
	}

	// Case‑insensitive match against options.
	for _, opt := range pq.Options {
		if strings.EqualFold(input, opt) {
			hl.Answer(opt)
			return true
		}
	}

	// Accept any input if no options defined.
	if len(pq.Options) == 0 {
		hl.Answer(input)
		return true
	}

	return false
}
