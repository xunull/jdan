package httptiming

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMeasure_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	r, err := Measure(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 {
		t.Fatalf("status = %d", r.StatusCode)
	}
	if r.DNSLookup < 0 {
		t.Fatalf("DNSLookup = %v", r.DNSLookup)
	}
	if r.TCPConnect < 0 {
		t.Fatalf("TCPConnect = %v", r.TCPConnect)
	}
	if r.TLSHandshake != 0 {
		t.Fatalf("TLSHandshake should be 0 for HTTP, got %v", r.TLSHandshake)
	}
	if r.ServerProcessing <= 0 {
		t.Fatalf("ServerProcessing = %v", r.ServerProcessing)
	}
	if r.ContentTransfer < 0 {
		t.Fatalf("ContentTransfer = %v", r.ContentTransfer)
	}
	if r.Total <= 0 {
		t.Fatalf("Total = %v", r.Total)
	}
}
