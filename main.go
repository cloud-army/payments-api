package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requests HTTP",
	}, []string{"method", "path", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duración de requests HTTP",
		Buckets: prometheus.DefBuckets,
	}, []string{"path"})

	paymentsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payments_processed_total",
		Help: "Total de pagos procesados",
	})

	paymentAmount = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "payment_amount_usd",
		Help:    "Monto de pagos en USD",
		Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000},
	})

	startTime = time.Now()
)

type Payment struct {
	ID       string  `json:"id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
	Method   string  `json:"method"`
	Created  string  `json:"created_at"`
}

func instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		defer timer.ObserveDuration()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		w.Header().Set("Content-Type", "application/json")
		next(rw, r)
		httpRequests.WithLabelValues(r.Method, path, http.StatusText(rw.status)).Inc()
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Simula tráfico de fondo para que las métricas sean interesantes
	go func() {
		for {
			amt := rand.Float64() * 5000
			paymentAmount.Observe(amt)
			paymentsProcessed.Inc()
			time.Sleep(time.Duration(2+rand.Intn(8)) * time.Second)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", instrument("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"app":    "payments-api",
			"uptime": time.Since(startTime).String(),
		})
	}))

	mux.HandleFunc("/api/v1/payments", instrument("/api/v1/payments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			payments := []Payment{
				{ID: "pay_001", Amount: 1250.00, Currency: "USD", Status: "completed", Method: "card", Created: "2026-08-14T10:00:00Z"},
				{ID: "pay_002", Amount: 89.99, Currency: "USD", Status: "completed", Method: "bank_transfer", Created: "2026-08-14T11:30:00Z"},
				{ID: "pay_003", Amount: 3400.00, Currency: "USD", Status: "pending", Method: "card", Created: "2026-08-14T12:00:00Z"},
			}
			json.NewEncoder(w).Encode(payments)
		case http.MethodPost:
			var p Payment
			json.NewDecoder(r.Body).Decode(&p)
			p.ID = "pay_" + time.Now().Format("20060102150405")
			p.Status = "completed"
			p.Created = time.Now().UTC().Format(time.RFC3339)
			paymentAmount.Observe(p.Amount)
			paymentsProcessed.Inc()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(p)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/v1/payments/stats", instrument("/api/v1/payments/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"total_today":   42,
			"volume_usd":    87340.50,
			"success_rate":  0.987,
			"avg_amount":    2079.77,
			"uptime":        time.Since(startTime).String(),
		})
	}))

	mux.Handle("/metrics", promhttp.Handler())

	log.Printf("payments-api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
