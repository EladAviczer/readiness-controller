package controller

import (
	"context"
	"log"
	"time"

	"readiness-controller/internal/config"
	"readiness-controller/internal/prober"
	"readiness-controller/internal/ui"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type ReadinessController struct {
	client *kubernetes.Clientset
	rule   config.GateRule
	probe  prober.Prober
}

func New(client *kubernetes.Clientset, rule config.GateRule, p prober.Prober) *ReadinessController {
	return &ReadinessController{client: client, rule: rule, probe: p}
}

func (c *ReadinessController) Start(ctx context.Context) {
	log.Printf("[%s] Started watching %s", c.rule.Name, c.rule.TargetLabel)

	interval := config.ParseInterval(c.rule.Interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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

func (c *ReadinessController) reconcile(ctx context.Context) {
	isHealthy := c.probe.Check()

	ui.UpdateState(c.rule.Name, c.rule.CheckTarget, c.rule.CheckType, isHealthy)

	targetStatus := corev1.ConditionFalse
	if isHealthy {
		targetStatus = corev1.ConditionTrue
	}

	pods, err := c.client.CoreV1().Pods(c.rule.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.rule.TargetLabel,
	})
	if err != nil {
		log.Printf("[%s] Failed to list pods: %v", c.rule.Name, err)
		return
	}

	for _, pod := range pods.Items {
		c.ensurePodGate(ctx, &pod, targetStatus)
	}
}

func (c *ReadinessController) ensurePodGate(ctx context.Context, pod *corev1.Pod, status corev1.ConditionStatus) {
	if isGateAlreadySet(pod, c.rule.GateName, status) {
		return
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		p, err := c.client.CoreV1().Pods(c.rule.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		updateCondition(p, c.rule.GateName, status)
		_, err = c.client.CoreV1().Pods(c.rule.Namespace).UpdateStatus(ctx, p, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		log.Printf("[%s] Failed update pod %s: %v", c.rule.Name, pod.Name, err)
	} else {
		log.Printf("[%s] Updated pod %s gate to %s", c.rule.Name, pod.Name, status)
	}
}

func updateCondition(pod *corev1.Pod, gateName string, status corev1.ConditionStatus) {
	for i, c := range pod.Status.Conditions {
		if c.Type == corev1.PodConditionType(gateName) {
			if c.Status != status {
				pod.Status.Conditions[i].Status = status
				pod.Status.Conditions[i].LastTransitionTime = metav1.Now()
				pod.Status.Conditions[i].Message = "Updated by Readiness Controller"
			}
			return
		}
	}
	pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
		Type: corev1.PodConditionType(gateName), Status: status, LastTransitionTime: metav1.Now(), Reason: "ControllerCheck", Message: "Updated by Readiness Controller",
	})
}

func isGateAlreadySet(pod *corev1.Pod, gateName string, status corev1.ConditionStatus) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodConditionType(gateName) {
			return c.Status == status
		}
	}
	return false
}
