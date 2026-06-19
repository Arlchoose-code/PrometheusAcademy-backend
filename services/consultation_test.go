package services

import "testing"

func TestConsultationSlotFromPayloadAcceptsDateOnly(t *testing.T) {
	slot, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18",
		TimeStart:   "10:00",
		TimeEnd:     "11:00",
		IsAvailable: true,
	})
	if err != nil {
		t.Fatalf("expected date-only payload to be valid: %v", err)
	}
	if got := slot.Date.Format("2006-01-02"); got != "2026-06-18" {
		t.Fatalf("expected normalized date 2026-06-18, got %s", got)
	}
	if slot.TimeStart != "10:00" || slot.TimeEnd != "11:00" || !slot.IsAvailable {
		t.Fatalf("slot fields were not mapped correctly: %#v", slot)
	}
}

func TestConsultationSlotFromPayloadAcceptsRFC3339(t *testing.T) {
	slot, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18T00:00:00Z",
		TimeStart:   "10:00",
		TimeEnd:     "11:00",
		IsAvailable: true,
	})
	if err != nil {
		t.Fatalf("expected RFC3339 payload to be valid: %v", err)
	}
	if got := slot.Date.Format("2006-01-02"); got != "2026-06-18" {
		t.Fatalf("expected normalized date 2026-06-18, got %s", got)
	}
}

func TestConsultationSlotFromPayloadDefaultsCapacityToOne(t *testing.T) {
	slot, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18",
		TimeStart:   "10:00",
		TimeEnd:     "11:00",
		IsAvailable: true,
	})
	if err != nil {
		t.Fatalf("expected slot payload to be valid: %v", err)
	}
	if slot.Capacity != 1 {
		t.Fatalf("expected capacity 1, got %d", slot.Capacity)
	}
}

func TestConsultationSlotFromPayloadAcceptsTeamCapacity(t *testing.T) {
	slot, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18",
		TimeStart:   "10:00",
		TimeEnd:     "11:00",
		Capacity:    3,
		IsAvailable: true,
	})
	if err != nil {
		t.Fatalf("expected slot payload to be valid: %v", err)
	}
	if slot.Capacity != 3 {
		t.Fatalf("expected capacity 3, got %d", slot.Capacity)
	}
}

func TestConsultationSlotFromPayloadRejectsInvalidTimeRange(t *testing.T) {
	_, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18",
		TimeStart:   "11:00",
		TimeEnd:     "10:00",
		IsAvailable: true,
	})
	if err == nil {
		t.Fatal("expected an invalid time range error")
	}
}

func TestConsultationSlotFromPayloadRejectsExcessiveCapacity(t *testing.T) {
	_, err := ConsultationSlotFromPayload(ConsultationSlotPayload{
		Date:        "2026-06-18",
		TimeStart:   "10:00",
		TimeEnd:     "11:00",
		Capacity:    101,
		IsAvailable: true,
	})
	if err == nil {
		t.Fatal("expected a capacity validation error")
	}
}
