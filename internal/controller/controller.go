package controller

import (
	"context"
	"log"
	"time"

	"heartbeat-operator/api/v1alpha1"
	"heartbeat-operator/internal/config"
	"heartbeat-operator/internal/metrics"
	"heartbeat-operator/internal/prober"
	"heartbeat-operator/internal/ui"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

type ReadinessController struct {
	client    *kubernetes.Clientset
	crdClient *CrdClient
	rule      config.GateRule
	probe     prober.Prober
	recorder  record.EventRecorder
}

// New creates a new ReadinessController
func New(client *kubernetes.Clientset, crdClient *CrdClient, rule config.GateRule, p prober.Prober, recorder record.EventRecorder) *ReadinessController {
	return &ReadinessController{
		client:    client,
		crdClient: crdClient,
		rule:      rule,
		probe:     p,
		recorder:  recorder,
	}
}

func (c *ReadinessController) Start(ctx context.Context) {
	log.Printf("[%s] Started watching %s (Targeting CRD)", c.rule.Name, c.rule.TargetLabel)

	// Ensure CR exists
	err := c.ensureCR(ctx)
	if err != nil {
		log.Printf("[%s] Failed to ensure CRD: %v", c.rule.Name, err)
		// Don't exit, maybe CRD isn't installed yet, retry in loop
	}

	interval := config.ParseInterval(c.rule.Interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check
	c.reconcile(ctx)

	for {
		select {
		case <-ticker.C:
			c.reconcile(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *ReadinessController) ensureCR(ctx context.Context) error {
	// Check if already exists
	_, err := c.crdClient.Get(ctx, c.rule.Name)
	if err == nil {
		return nil // Exists
	}

	// Create if not exists
	dc := &v1alpha1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.rule.Name,
			Namespace: c.rule.Namespace,
		},
		Spec: v1alpha1.ProbeSpec{
			CheckType:   c.rule.CheckType,
			CheckTarget: c.rule.CheckTarget,
			Interval:    c.rule.Interval,
		},
	}
	log.Printf("[%s] Creating Probe CR...", c.rule.Name)
	_, err = c.crdClient.Create(ctx, dc)
	return err
}

func (c *ReadinessController) reconcile(ctx context.Context) {
	start := time.Now()
	isHealthy := c.probe.Check()
	duration := time.Since(start).Seconds()

	metrics.ProbeDuration.WithLabelValues(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType).Observe(duration)
	metrics.ProbeLastTimestamp.WithLabelValues(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType).Set(float64(time.Now().Unix()))

	if isHealthy {
		metrics.ProbeSuccess.WithLabelValues(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType).Set(1)
	} else {
		metrics.ProbeSuccess.WithLabelValues(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType).Set(0)
	}

	ui.UpdateState(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType, isHealthy)

	// Fetch current CR to update status
	cr, err := c.crdClient.Get(ctx, c.rule.Name)
	if err != nil {
		// Try to re-create if missing?
		if err := c.ensureCR(ctx); err != nil {
			log.Printf("[%s] CR missing and failed to create: %v", c.rule.Name, err)
			return
		}
		// Fetch again
		cr, err = c.crdClient.Get(ctx, c.rule.Name)
		if err != nil {
			log.Printf("[%s] Failed to get CR: %v", c.rule.Name, err)
			return
		}
	}

	// Update status logic
	// Only update if changed or if it's been a while?
	// For now, simple update
	now := metav1.Now()
	msg := "Check passed"
	if !isHealthy {
		msg = "Check failed"
	}

	if cr.Status.Healthy != isHealthy || cr.Status.Message != msg {
		cr.Status.Healthy = isHealthy
		cr.Status.Message = msg
		cr.Status.LastProbeTime = &now
		_, err := c.crdClient.UpdateStatus(ctx, cr)
		if err != nil {
			log.Printf("[%s] Failed to update CR status: %v", c.rule.Name, err)
		} else {
			log.Printf("[%s] Updated CR status: healthy=%v", c.rule.Name, isHealthy)
		}
	} else {
		// Just update timestamp periodically? Or leave it to reduce API load?
		// Let's update timestamp if it's been > 1 minute or if we want liveliness
		if cr.Status.LastProbeTime == nil || time.Since(cr.Status.LastProbeTime.Time) > time.Minute {
			cr.Status.LastProbeTime = &now
			if _, err := c.crdClient.UpdateStatus(ctx, cr); err != nil {
				log.Printf("[%s] Failed to update CR timestamp: %v", c.rule.Name, err)
			}
		}
	}
}
