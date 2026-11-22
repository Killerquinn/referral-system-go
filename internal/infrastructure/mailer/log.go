// Package mailer sends transactional emails (password reset codes, later email verification).
//
// LogMailer is the dev implementation: it only writes the email to the log, so you can copy
// the code from the console. In production it gets replaced by a real provider (Resend) that
// implements the same methods — callers (the queue workers) don't change.
package mailer

import (
	"context"

	"go.uber.org/zap"
)

type LogMailer struct {
	log *zap.Logger
}

func NewLogMailer(log *zap.Logger) *LogMailer {
	return &LogMailer{log: log.With(zap.String("component", "mailer"))}
}

// SendResetCode implements queue.MailerProcess.
// The code is logged in plain text: dev only, never use LogMailer in production.
func (m *LogMailer) SendResetCode(_ context.Context, email, code string) error {
	m.log.Info("DEV MAIL: password reset code",
		zap.String("to", email),
		zap.String("code", code),
	)
	return nil
}
