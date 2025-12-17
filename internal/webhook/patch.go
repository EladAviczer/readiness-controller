package webhook

import (
	"context"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func PatchWebhookCABundle(client kubernetes.Interface, webhookConfigName string, caCertPEM []byte) error {
	ctx := context.TODO()

	webhookConfig, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, webhookConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get MutatingWebhookConfiguration %s: %v", webhookConfigName, err)
	}

	log.Printf("Injecting CA Bundle into Webhook Configuration: %s", webhookConfigName)

	for i := range webhookConfig.Webhooks {
		webhookConfig.Webhooks[i].ClientConfig.CABundle = caCertPEM
	}

	_, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch MutatingWebhookConfiguration with CA bundle: %v", err)
	}

	log.Println("Successfully patched Webhook CA Bundle.")
	return nil
}
