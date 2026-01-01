package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"heartbeat-operator/internal/config"
	"heartbeat-operator/internal/controller"
	"heartbeat-operator/internal/prober"
	"heartbeat-operator/internal/ui"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/config/gates.json"
	}

	rules, err := config.LoadRules(configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", configPath, err)
	}
	log.Printf("Loaded %d gate rules", len(rules))

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get k8s config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}

	log.Println("Initializing Event Broadcaster...")
	eventBroadcaster := record.NewBroadcaster()

	eventBroadcaster.StartStructuredLogging(0)

	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: clientset.CoreV1().Events(""),
	})
	defer eventBroadcaster.Shutdown()

	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: "readiness-controller"},
	)
	// -------------------------------------------------------

	ui.Start("8080")

	// Start Metrics Server
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":9090"
	}
	// Basic check to see if we should enable it, or just always enable it if env var is present?
	// For now, always enable on 9090 unless configured otherwise.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		server := &http.Server{
			Addr:    metricsAddr,
			Handler: mux,
		}
		log.Printf("Starting metrics server on %s", metricsAddr)
		if err := server.ListenAndServe(); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	for _, rule := range rules {
		wg.Add(1)

		r := rule

		go func() {
			defer wg.Done()

			var p prober.Prober
			switch r.CheckType {
			case "http":
				p = prober.NewHttpProber(r.CheckTarget)
			case "tcp":
				p = prober.NewTcpProber(r.CheckTarget)
			case "exec":
				p = prober.NewExecProber(r.CheckTarget)
			default:
				log.Printf("[%s] Unknown CheckType '%s', skipping rule", r.Name, r.CheckType)
				return
			}

			// -------------------------------------------------------
			// Initialize CRD Client
			crdClient, err := controller.NewCrdClient(k8sConfig, r.Namespace)
			if err != nil {
				log.Printf("Failed to create CRD client: %v", err)
				return
			}

			ctrl := controller.New(clientset, crdClient, r, p, recorder)
			ctrl.Start(ctx)
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
	cancel()
	wg.Wait()
}
