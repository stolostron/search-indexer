// Copyright Contributors to the Open Cluster Management project

package server

import (
	"context"
	"crypto/tls"
	"github.com/segmentio/kafka-go"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/database"
	"github.com/stolostron/search-indexer/pkg/metrics"
	"k8s.io/klog/v2"
)

type ServerConfig struct {
	Dao *database.DAO
}

func (s *ServerConfig) StartAndListen(ctx context.Context) {
	router := mux.NewRouter()
	router.HandleFunc("/liveness", LivenessProbe).Methods("GET")
	router.HandleFunc("/readiness", ReadinessProbe).Methods("GET")
	router.Handle("/metrics", promhttp.HandlerFor(metrics.PromRegistry, promhttp.HandlerOpts{})).Methods("GET")

	// Add middleware to the /aggregator subroute.
	syncSubrouter := router.PathPrefix("/aggregator").Subrouter()
	syncSubrouter.Use(metrics.PrometheusMiddleware)
	syncSubrouter.Use(requestLimiterMiddleware)
	syncSubrouter.Use(largeRequestLimiterMiddleware)
	syncSubrouter.HandleFunc("/clusters/{id}/sync", s.SyncResources).Methods("POST")

	// Configure TLS
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
	srv := &http.Server{
		Addr:              config.Cfg.ServerAddress,
		Handler:           router,
		TLSConfig:         cfg,
		ReadHeaderTimeout: time.Duration(config.Cfg.HTTPTimeout) * time.Millisecond,
		ReadTimeout:       time.Duration(config.Cfg.HTTPTimeout) * time.Millisecond,
		WriteTimeout:      time.Duration(config.Cfg.HTTPTimeout) * time.Millisecond,
		TLSNextProto:      make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	// Start the server
	go func() {
		klog.Info("Listening on: ", srv.Addr)
		// ErrServerClosed is returned on graceful close.
		if err := srv.ListenAndServeTLS("./sslcert/tls.crt", "./sslcert/tls.key"); err != http.ErrServerClosed {
			if config.Cfg.DevelopmentMode {
				klog.Fatal(err, ". If missing certificates in development mode, use ./setup.sh to generate.")
			} else {
				klog.Fatal(err, ". Encountered while starting the server.")
			}
		}
	}()

	for i := 0; i < 6; i++ {

		// Consume kafka resource messages
		go func(ctx context.Context, i int) {
			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers:     []string{"kafka-kafka-bootstrap.amq-streams.svc:9092"},
				Topic:       "resource-events",
				StartOffset: kafka.LastOffset,
				Partition:   i,
			})
			defer r.Close()
			s.KafkaResourceHandler(ctx, r, i)
		}(ctx, i)
	}

	// Wait for cancel signal
	<-ctx.Done()
	klog.Warning("Stopping the server.")
	ctxWithTimeout, ctxCancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
	if err := srv.Shutdown(ctxWithTimeout); err != nil {
		klog.Error("Encountered error stopping the server. ", err)
	} else {
		klog.Warning("Server stopped.")
	}
	ctxCancel()
}
