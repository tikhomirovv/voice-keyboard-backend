package pkg

import (
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

const (
	// EmailDefaultLocale      = "en"
	EmailTypeConfirmation   = "email_confirmation"
	EmailTypeForgotPassword = "pass_restore"
)

type Emailer struct {
	cfg *Config
	lg  logger.Logger
}

type EmailMessage struct {
	UserID    int
	Email     string
	Variables map[string]any
}

func NewEmailer(cfg *Config, lg logger.Logger) *Emailer {
	return &Emailer{
		cfg: cfg,
		lg:  lg,
	}
}

func (e *Emailer) Send(messageType string, messages []EmailMessage) error {
	e.lg.Debug("[Emailer] Send", "type", messageType, "messages", messages)
	return nil
}

func (e *Emailer) SendConfirmationEmail(userId int, email string, confirmationLink string) error {
	return e.Send(EmailTypeConfirmation, []EmailMessage{
		{
			UserID: userId,
			Email:  email,
			Variables: map[string]any{
				"email_verification_link": confirmationLink,
			},
		},
	})
}

func (e *Emailer) SendForgotPasswordEmail(userId int, email string, resetPasswordLink string) error {
	return e.Send(EmailTypeForgotPassword, []EmailMessage{
		{
			UserID: userId,
			Email:  email,
			Variables: map[string]any{
				"pass_restore_link": resetPasswordLink,
			},
		},
	})
}
