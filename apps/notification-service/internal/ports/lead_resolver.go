package ports

import "context"

type LeadWelcomeRecipient struct {
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	FirstName        string `json:"first_name"`
	Eligible         bool   `json:"eligible"`
	EmailEligible    bool   `json:"email_eligible"`
	SMSEligible      bool   `json:"sms_eligible"`
	WhatsAppEligible bool   `json:"whatsapp_eligible"`
}

type LeadResolver interface {
	ResolveWelcomeRecipient(ctx context.Context, tenantID, leadID string) (LeadWelcomeRecipient, error)
}
