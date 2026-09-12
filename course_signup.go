package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"time"
)

type Signup struct {
	LearnerEmail string `json:"learner_email"`
	CourseID     string `json:"course_id"`
	VerifyURL    string `json:"verify_url"`
	DeadlineDays int    `json:"deadline_days"`
}

type Enrollment struct {
	CourseID     string    `json:"course_id"`
	LearnerEmail string    `json:"learner_email"`
	MessageID    string    `json:"message_id"`
	CourseOpens  time.Time `json:"course_opens"`
	Deadline     time.Time `json:"deadline"`
	State        string    `json:"state"`
}

type EducatorReport struct {
	CourseID     string          `json:"course_id"`
	LearnerEmail string          `json:"learner_email"`
	MessageID    string          `json:"message_id"`
	Deadline     time.Time       `json:"deadline"`
	State        string          `json:"state"`
	Delivery     json.RawMessage `json:"delivery"`
}

type SignupWorkflow struct {
	Email EmailClient
	Now   func() time.Time
}

func (w SignupWorkflow) Start(ctx context.Context, input Signup) (Enrollment, error) {
	if input.LearnerEmail == "" || input.CourseID == "" || input.VerifyURL == "" || input.DeadlineDays < 1 {
		return Enrollment{}, fmt.Errorf("learner_email, course_id, verify_url, and a positive deadline_days are required")
	}
	if _, err := url.ParseRequestURI(input.VerifyURL); err != nil {
		return Enrollment{}, fmt.Errorf("verify_url: %w", err)
	}
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	deadline := now.AddDate(0, 0, input.DeadlineDays)
	subject := "Verify your email for course " + input.CourseID
	body := fmt.Sprintf("<p>Confirm your learner email to open <strong>%s</strong>.</p><p><a href=\"%s\">Verify email</a></p><p>Course deadline: %s</p>", html.EscapeString(input.CourseID), html.EscapeString(input.VerifyURL), deadline.Format(time.RFC3339))
	keyBytes := sha256.Sum256([]byte(input.CourseID + "\x00" + input.LearnerEmail + "\x00" + input.VerifyURL))
	messageID, err := w.Email.SendVerification(ctx, input.LearnerEmail, subject, body, "signup-"+hex.EncodeToString(keyBytes[:16]))
	if err != nil {
		return Enrollment{}, err
	}
	return Enrollment{CourseID: input.CourseID, LearnerEmail: input.LearnerEmail, MessageID: messageID, CourseOpens: now, Deadline: deadline, State: "pending_email_verification"}, nil
}

func (w SignupWorkflow) Report(ctx context.Context, enrollment Enrollment) (EducatorReport, error) {
	delivery, err := w.Email.GetEmail(ctx, enrollment.MessageID)
	if err != nil {
		return EducatorReport{}, err
	}
	return EducatorReport{CourseID: enrollment.CourseID, LearnerEmail: enrollment.LearnerEmail, MessageID: enrollment.MessageID, Deadline: enrollment.Deadline, State: enrollment.State, Delivery: delivery}, nil
}
