package useragent_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artyom/useragent"
)

func TestSet(t *testing.T) {
	const agent = "CustomRobot/1.0"
	var agentSeen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentSeen = r.UserAgent()
	}))
	defer srv.Close()
	client := &http.Client{
		Transport: useragent.Set(http.DefaultTransport, agent),
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if agentSeen != agent {
		t.Fatalf("agent seen: %q, want %q", agentSeen, agent)
	}
}

func TestTransportVisibility(t *testing.T) {
	tr := useragent.Set(&http.Transport{}, "agent")
	if _, ok := tr.(interface {
		CloseIdleConnections()
	}); !ok {
		t.Fatal("http.Transport methods are not visible")
	}
}
