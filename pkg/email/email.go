package email

import (
	"context"

	brevo "github.com/getbrevo/brevo-go/lib"
)

type Sender struct {
	client *brevo.APIClient
}

func NewEmailSender(token string) Sender {
	cfg := brevo.NewConfiguration()
	cfg.AddDefaultHeader("api-key", token)

	return Sender{client: brevo.NewAPIClient(cfg)}
}

func (s *Sender) baseEmail(ctx context.Context, templateID int64, to []string, link *string) (string, error) {
	var params any
	if link != nil {
		params = map[string]string{"LINK": *link}
	}

	emailReceivers := make([]brevo.SendSmtpEmailTo, 0, len(to))
	for _, t := range to {
		if t == "" {
			continue
		}
		emailReceivers = append(emailReceivers, brevo.SendSmtpEmailTo{Email: t})
	}

	em, _, err := s.client.TransactionalEmailsApi.SendTransacEmail(ctx, brevo.SendSmtpEmail{
		To:         emailReceivers,
		TemplateId: templateID,
		Params:     &params,
	})

	return em.MessageId, err
}
