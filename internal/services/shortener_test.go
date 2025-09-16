func TestCreateShortURLTracing(t *testing.T) {
    repo := &mockRepository{}
    svc := services.NewShortenerService(repo)
    h := handlers.NewHandler(svc, repo)
    req, _ := http.NewRequest("POST", "/newurl", strings.NewReader(`{"domain":"shortenurl.org","url":"https://google.com"}`))
    rr := httptest.NewRecorder()

    tracerProvider := trace.NewTracerProvider()
    otel.SetTracerProvider(tracerProvider)
    h.CreateShortURL(rr, req)

    // Verify spans in test collector (mock or Jaeger)
}
