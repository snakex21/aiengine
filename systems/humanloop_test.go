package systems

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A question posted over HTTP reaches OnAsk on the server goroutine, and an
// answer typed in the console resolves the request (run with -race).
func TestHumanLoopAskAndAnswer(t *testing.T) {
	hl := NewHumanLoop()
	asked := make(chan string, 1)
	hl.OnAsk = func(q string, _ []string) { asked <- q }
	if err := hl.Start(0); err != nil {
		t.Fatal(err)
	}
	defer hl.Stop()
	if !hl.Active() {
		t.Fatal("server not active")
	}
	got := make(chan string, 1)
	go func() {
		resp, err := http.Post("http://"+hl.Addr()+"/ask", "application/json",
			strings.NewReader(`{"question":"ok?","options":["tak","nie"]}`))
		if err != nil {
			got <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		var out map[string]string
		json.NewDecoder(resp.Body).Decode(&out)
		got <- out["answer"]
	}()
	select {
	case q := <-asked:
		if q != "ok?" {
			t.Fatalf("question %q", q)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnAsk not called")
	}
	if !hl.AnswerFromConsole("TAK") {
		t.Fatal("console answer not accepted")
	}
	if a := <-got; a != "tak" {
		t.Fatalf("answer %q", a)
	}
}
