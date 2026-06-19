package services

import "testing"

func TestGenerateTalentReviewToken(t *testing.T) {
	first, firstHash, err := GenerateTalentReviewToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, secondHash, err := GenerateTalentReviewToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == "" || firstHash == "" {
		t.Fatal("token and hash must not be empty")
	}
	if first == second || firstHash == secondHash {
		t.Fatal("generated invitations must use unique tokens")
	}
	if firstHash != HashTalentReviewToken(first) {
		t.Fatal("stored hash must match the raw token")
	}
	if first == firstHash {
		t.Fatal("raw token must not be stored as its hash")
	}
}

func TestTalentReviewStatusEligible(t *testing.T) {
	tests := []struct {
		applicationType string
		status          string
		want            bool
	}{
		{"job", "accepted", true},
		{"job", "completed", true},
		{"job", "reviewed", false},
		{"plus", "placed", true},
		{"plus", "completed", true},
		{"plus", "contacted", false},
		{"other", "completed", false},
	}
	for _, test := range tests {
		if got := TalentReviewStatusEligible(test.applicationType, test.status); got != test.want {
			t.Fatalf("eligibility %s/%s = %v, want %v", test.applicationType, test.status, got, test.want)
		}
	}
}
