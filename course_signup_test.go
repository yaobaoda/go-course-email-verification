package verification

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type recordingEmail struct {
	sentTo      string
	sentKey     string
	returnedID  string
	requestedID string
}

func (f *recordingEmail) SendVerification(_ context.Context, to, _, _, key string) (string, error) {
	f.sentTo, f.sentKey = to, key
	return f.returnedID, nil
}

func (f *recordingEmail) GetEmail(_ context.Context, messageID string) (json.RawMessage, error) {
	f.requestedID = messageID
	return json.RawMessage(`{"status":"delivered"}`), nil
}

func TestSignupHandoffToEducatorReport(t *testing.T) {
	fixed := time.Date(2026, time.August, 15, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		days int
		want time.Time
	}{
		{name: "one week course", days: 7, want: fixed.AddDate(0, 0, 7)},
		{name: "month course", days: 30, want: fixed.AddDate(0, 0, 30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mail := &recordingEmail{returnedID: "msg-course-42"}
			workflow := SignupWorkflow{Email: mail, Now: func() time.Time { return fixed }}
			enrollment, err := workflow.Start(context.Background(), Signup{LearnerEmail: "learner@example.edu", CourseID: "go-operations", VerifyURL: "https://learn.example.edu/verify/token", DeadlineDays: tt.days})
			if err != nil {
				t.Fatal(err)
			}
			if enrollment.Deadline != tt.want || enrollment.State != "pending_email_verification" {
				t.Fatalf("unexpected enrollment: %+v", enrollment)
			}
			if mail.sentTo != "learner@example.edu" || mail.sentKey == "" {
				t.Fatalf("send boundary not populated: %+v", mail)
			}
			report, err := workflow.Report(context.Background(), enrollment)
			if err != nil {
				t.Fatal(err)
			}
			if mail.requestedID != enrollment.MessageID || report.MessageID != "msg-course-42" {
				t.Fatalf("send-to-report handoff lost message id: request=%q report=%q", mail.requestedID, report.MessageID)
			}
		})
	}
}
