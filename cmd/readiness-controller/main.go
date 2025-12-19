package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"readiness-controller/internal/config"
	"readiness-controller/internal/controller"
	"readiness-controller/internal/prober"
	"readiness-controller/internal/ui"
	"readiness-controller/internal/webhook"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	ui.Start("8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		startWebhook(ctx, clientset)
	}()

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

			ctrl := controller.New(clientset, r, p)
			ctrl.Start(ctx)
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	cancel()
	wg.Wait()
	log.Println("Bye!")
}

func startWebhook(ctx context.Context, clientset *kubernetes.Clientset) {
	log.Println("Initializing Webhook Logic...")

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "readiness-controller"
	}

	log.Printf("Generating Certs for Service: %s.%s.svc", serviceName, namespace)

	certs, err := webhook.GenerateCerts(serviceName, namespace)
	if err != nil {
		log.Printf("❌ Webhook failed: could not generate certs: %v", err)
		return
	}

	certFile := "/tmp/tls.crt"
	keyFile := "/tmp/tls.key"
	if err := os.WriteFile(certFile, certs.ServerCert, 0644); err != nil {
		log.Printf("❌ Webhook failed: writing cert file: %v", err)
		return
	}
	if err := os.WriteFile(keyFile, certs.ServerKey, 0600); err != nil {
		log.Printf("❌ Webhook failed: writing key file: %v", err)
		return
	}

	webhookConfigName := "readiness-controller-webhook"
	err = webhook.PatchWebhookCABundle(clientset, webhookConfigName, certs.CACert)
	if err != nil {
		log.Printf("⚠️  Failed to patch Webhook CA Bundle: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", webhook.HandleMutate)

	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	go func() {
		log.Println("✅ Webhook Server listening on :8443")
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ Webhook Server crashed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Stopping Webhook Server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Webhook shutdown error: %v", err)
	}
}
