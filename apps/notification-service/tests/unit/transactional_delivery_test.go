package unit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
)

type captureTransactionalNotifier struct{ message domain.Message }

func (n *captureTransactionalNotifier) Send(_ context.Context, message domain.Message) error {
	n.message = message
	return nil
}

type receiptTransactionalNotifier struct {
	calls             int
	message           domain.Message
	deliveryAddress   string
	provider          string
	providerMessageID string
}

func (n *receiptTransactionalNotifier) Send(ctx context.Context, message domain.Message) error {
	_, err := n.SendWithReceipt(ctx, message)
	return err
}

func (n *receiptTransactionalNotifier) SendWithReceipt(_ context.Context, message domain.Message) (ports.ProviderReceipt, error) {
	n.calls++
	n.message = message
	var ok bool
	n.deliveryAddress, ok = message.Metadata["delivery_address"].(string)
	if !ok {
		n.deliveryAddress = ""
	}
	provider := n.provider
	if provider == "" {
		provider = "resend"
	}
	providerMessageID := n.providerMessageID
	if providerMessageID == "" {
		providerMessageID = "5c58a558-cb1e-4f32-80f8-bc55c692f324"
	}
	return ports.ProviderReceipt{Provider: provider, MessageID: providerMessageID}, nil
}

func TestTransactionalEmailPersistsRedactedFailureWhenProviderMissing(t *testing.T) {
	const tenantID = "11111111-1111-1111-1111-111111111111"
	messages := newFakeMessageRepo()
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo())
	message, err := svc.DeliverTransactionalEmail(context.Background(), tenantID, "guardian@example.com", "password_reset", map[string]any{"reset_token": "secret-reset-token"})
	if err == nil || message == nil {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	if message.Status != string(domain.MessageStatusFailed) || strings.Contains(message.Body, "secret-reset-token") {
		t.Fatalf("transactional failure was not safely persisted: %+v", message)
	}
	stored := messages.all(tenantID)
	if len(stored) != 1 || strings.Contains(stored[0].Body, "secret-reset-token") {
		t.Fatalf("stored transactional failure=%+v", stored)
	}
}

func TestTransactionalPasswordResetUsesServerInvisibleTrustedLink(t *testing.T) {
	messages := newFakeMessageRepo()
	capture := &captureTransactionalNotifier{}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"email": capture}),
		application.WithPublicAppURL("https://app.auraedu.com"),
	)
	message, err := svc.DeliverTransactionalEmail(context.Background(), "release-school", "teacher@example.com", "password_reset", map[string]any{"reset_token": "secret+reset/token"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capture.message.Body, "https://app.auraedu.com/reset-password?tenant=release-school#token=secret%2Breset%2Ftoken") {
		t.Fatalf("provider body does not contain the trusted fragment link: %q", capture.message.Body)
	}
	if strings.Contains(message.Body, "secret+reset/token") || strings.Contains(messages.all("release-school")[0].Body, "secret+reset/token") {
		t.Fatal("reset token was persisted after delivery")
	}
}

func TestTransactionalInviteContainsAUsableTrustedLinkAndPersistsNoToken(t *testing.T) {
	const tenantID = "release-school"
	messages := newFakeMessageRepo()
	capture := &captureTransactionalNotifier{}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"email": capture}),
		application.WithPublicAppURL("https://app.auraedu.com"),
	)
	message, err := svc.DeliverTransactionalEmail(context.Background(), tenantID, "admin@example.com", "user_invite", map[string]any{
		"invite_token": "secret+invite/token", "role": "school_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capture.message.Body, "https://app.auraedu.com/accept-invite?tenant=release-school#token=secret%2Binvite%2Ftoken") {
		t.Fatalf("provider body does not contain the trusted encoded link: %q", capture.message.Body)
	}
	if strings.Contains(message.Body, "secret+invite/token") || strings.Contains(messages.all(tenantID)[0].Body, "secret+invite/token") {
		t.Fatal("invite token was persisted after delivery")
	}
}

func TestTransactionalEmailRecordsProviderAcceptanceAndRemovesRawAddress(t *testing.T) {
	messages := newFakeMessageRepo()
	receipt := &receiptTransactionalNotifier{}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"email": receipt}),
	)
	message, err := svc.DeliverTransactionalEmail(context.Background(), "release-school", "teacher@example.com", "password_reset", map[string]any{"reset_token": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.calls != 1 || message.Provider == nil || *message.Provider != "resend" || message.DeliveryStatus == nil || *message.DeliveryStatus != "accepted" {
		t.Fatalf("provider outcome message=%+v calls=%d", message, receipt.calls)
	}
	if _, retained := message.Metadata["delivery_address"]; retained || len(fmt.Sprint(message.Metadata["delivery_address_hash"])) != 64 {
		t.Fatalf("delivery metadata was not minimized: %+v", message.Metadata)
	}
}

func TestTwilioDeliveryRecordsReceiptAndPersistsOnlyAddressHash(t *testing.T) {
	const tenantID = "release-school"
	messages := newFakeMessageRepo()
	receipt := &receiptTransactionalNotifier{
		provider:          "twilio",
		providerMessageID: "SM0123456789abcdef0123456789abcdef",
	}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"sms": receipt}),
	)
	message, err := domain.NewMessage(tenantID, "guardian-id", "sms", "Attendance alert", "A student was marked absent.", nil,
		map[string]any{"delivery_address": "+233200000004"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(context.Background(), tenantID, message); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeliverScheduled(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if receipt.calls != 1 || receipt.deliveryAddress != "+233200000004" {
		t.Fatalf("provider delivery address=%q calls=%d", receipt.deliveryAddress, receipt.calls)
	}
	if message.Provider == nil || *message.Provider != "twilio" || message.DeliveryStatus == nil || *message.DeliveryStatus != "accepted" {
		t.Fatalf("provider outcome=%+v", message)
	}
	if _, retained := message.Metadata["delivery_address"]; retained || len(fmt.Sprint(message.Metadata["delivery_address_hash"])) != 64 {
		t.Fatalf("persisted SMS metadata was not minimized: %+v", message.Metadata)
	}
}

func TestTransactionalEmailSuppressionPreventsProviderIO(t *testing.T) {
	const tenantID = "release-school"
	messages := newFakeMessageRepo()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("teacher@example.com")))
	messages.suppressions[tenantID+"/"+hash] = true
	receipt := &receiptTransactionalNotifier{}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"email": receipt}),
	)
	message, err := svc.DeliverTransactionalEmail(context.Background(), tenantID, "teacher@example.com", "password_reset", map[string]any{"reset_token": "secret"})
	if err == nil || !strings.Contains(err.Error(), "suppressed") || receipt.calls != 0 {
		t.Fatalf("message=%+v calls=%d err=%v", message, receipt.calls, err)
	}
	if _, retained := message.Metadata["delivery_address"]; retained {
		t.Fatalf("suppressed address retained: %+v", message.Metadata)
	}
}

func TestConsentedEmailCarriesPrivateOptOutWithoutPersistingCredential(t *testing.T) {
	const tenantID = "release-school"
	messages := newFakeMessageRepo()
	receipt := &receiptTransactionalNotifier{}
	manager, err := application.NewUnsubscribeManager(strings.Repeat("u", 48), "https://auraedugh.vercel.app")
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithNotifiers(map[string]ports.Notifier{"email": receipt}),
		application.WithUnsubscribeManager(manager),
	)
	message, err := domain.NewMessage(tenantID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "email", "Admissions update", "Your application is moving forward", nil,
		map[string]any{"delivery_address": "teacher@example.com", "consent_verified": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(context.Background(), tenantID, message); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeliverScheduled(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(receipt.message.Body, "https://auraedugh.vercel.app/unsubscribe#token=") {
		t.Fatalf("provider email lacked opt-out: %q", receipt.message.Body)
	}
	if strings.Contains(message.Body, "unsubscribe#token=") || strings.Contains(messages.all(tenantID)[0].Body, "unsubscribe#token=") {
		t.Fatal("unsubscribe credential was persisted")
	}
	linkStart := strings.Index(receipt.message.Body, "https://auraedugh.vercel.app/unsubscribe#token=")
	if linkStart < 0 {
		t.Fatal("provider email lacked a parseable opt-out link")
	}
	link := receipt.message.Body[linkStart:]
	token := strings.TrimPrefix(link, "https://auraedugh.vercel.app/unsubscribe#token=")
	if err := svc.UnsubscribeEmail(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("teacher@example.com")))
	if !messages.suppressions[tenantID+"/"+hash] {
		t.Fatal("email opt-out was not persisted")
	}
}

func TestValidatePublicAppURLFailsClosedInProduction(t *testing.T) {
	for _, value := range []string{"http://app.auraedu.com", "https://localhost:3000", "https://app.auraedu.example", "https://user:secret@app.auraedu.com", "https://app.auraedu.com/path"} {
		if err := application.ValidatePublicAppURL(value, true); err == nil {
			t.Fatalf("unsafe production origin accepted: %s", value)
		}
	}
	if err := application.ValidatePublicAppURL("https://app.auraedu.com", true); err != nil {
		t.Fatalf("valid production origin rejected: %v", err)
	}
}
