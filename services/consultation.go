package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ConsultationRescheduleLimitKey = "consultation_reschedule_limit"

type ConsultationSlotRow struct {
	ID                uint      `json:"id"`
	OwnerID           uint      `json:"owner_id"`
	OwnerName         string    `json:"owner_name"`
	Date              time.Time `json:"date"`
	TimeStart         string    `json:"time_start"`
	TimeEnd           string    `json:"time_end"`
	Capacity          int       `json:"capacity"`
	ActiveBookings    int       `json:"active_bookings"`
	AvailableCapacity int       `json:"available_capacity"`
	IsAvailable       bool      `json:"is_available"`
	IsBooked          bool      `json:"is_booked"`
}

type ConsultationBookingRow struct {
	ID              uint      `json:"id"`
	SlotID          uint      `json:"slot_id"`
	OwnerID         uint      `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	UserID          uint      `json:"user_id"`
	UserName        string    `json:"user_name"`
	UserEmail       string    `json:"user_email"`
	OrderID         uint      `json:"order_id"`
	Product         string    `json:"product"`
	Date            time.Time `json:"date"`
	TimeStart       string    `json:"time_start"`
	TimeEnd         string    `json:"time_end"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes"`
	RescheduleCount int       `json:"reschedule_count"`
	RescheduleLimit int       `json:"reschedule_limit"`
	CreatedAt       time.Time `json:"created_at"`
}

type ConsultationBookingRequirement struct {
	OwnerID uint
	Service string
}

type ConsultationSlotPayload struct {
	OwnerID     uint   `json:"owner_id"`
	Date        string `json:"date"`
	TimeStart   string `json:"time_start"`
	TimeEnd     string `json:"time_end"`
	Capacity    int    `json:"capacity"`
	IsAvailable bool   `json:"is_available"`
}

func ConsultationSlotFromPayload(payload ConsultationSlotPayload) (models.ConsultationSlot, error) {
	date, err := parseConsultationDate(payload.Date)
	if err != nil {
		return models.ConsultationSlot{}, err
	}
	timeStart := strings.TrimSpace(payload.TimeStart)
	timeEnd := strings.TrimSpace(payload.TimeEnd)
	if timeStart == "" || timeEnd == "" {
		return models.ConsultationSlot{}, errors.New("date, start time, and end time are required")
	}
	start, startErr := time.Parse("15:04", timeStart)
	end, endErr := time.Parse("15:04", timeEnd)
	if startErr != nil || endErr != nil || !end.After(start) {
		return models.ConsultationSlot{}, errors.New("end time must be after start time")
	}
	capacity := payload.Capacity
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 100 {
		return models.ConsultationSlot{}, errors.New("capacity cannot exceed 100")
	}
	return models.ConsultationSlot{
		OwnerID:     payload.OwnerID,
		Date:        date,
		TimeStart:   timeStart,
		TimeEnd:     timeEnd,
		Capacity:    capacity,
		IsAvailable: payload.IsAvailable,
	}, nil
}

func ValidateConsultationSlotOverlap(ctx context.Context, db *gorm.DB, slot models.ConsultationSlot, ignoreID uint) error {
	if slot.OwnerID == 0 {
		return nil
	}
	var count int64
	query := db.WithContext(ctx).Model(&models.ConsultationSlot{}).
		Where("owner_id = ? AND date = ? AND time_start < ? AND time_end > ?", slot.OwnerID, slot.Date, slot.TimeEnd, slot.TimeStart)
	if ignoreID > 0 {
		query = query.Where("id <> ?", ignoreID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("This provider already has an overlapping consultation slot")
	}
	return nil
}

func ValidateConsultationSlotCapacity(ctx context.Context, db *gorm.DB, slotID uint, capacity int) error {
	if slotID == 0 {
		return nil
	}
	var activeBookings int64
	if err := db.WithContext(ctx).Model(&models.ConsultationBooking{}).
		Where("slot_id = ? AND status IN ?", slotID, []string{"pending", "confirmed"}).
		Count(&activeBookings).Error; err != nil {
		return err
	}
	if activeBookings > int64(capacity) {
		return fmt.Errorf("Capacity cannot be lower than the current active booking count")
	}
	return nil
}

func CreateConsultationSlotRecord(ctx context.Context, db *gorm.DB, slot *models.ConsultationSlot) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if slot.OwnerID != 0 {
			var owner models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&owner, slot.OwnerID).Error; err != nil {
				return fmt.Errorf("Slot owner not found")
			}
		}
		if err := ValidateConsultationSlotOverlap(ctx, tx, *slot, 0); err != nil {
			return err
		}
		return tx.Create(slot).Error
	})
}

func UpdateConsultationSlotRecord(ctx context.Context, db *gorm.DB, slotID uint, ownerConstraint *uint, slot models.ConsultationSlot) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.ConsultationSlot
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", slotID)
		if ownerConstraint != nil {
			query = query.Where("owner_id = ?", *ownerConstraint)
		}
		if err := query.First(&current).Error; err != nil {
			return fmt.Errorf("Slot not found")
		}
		if slot.OwnerID != 0 {
			var owner models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&owner, slot.OwnerID).Error; err != nil {
				return fmt.Errorf("Slot owner not found")
			}
		}
		if err := ValidateConsultationSlotOverlap(ctx, tx, slot, slotID); err != nil {
			return err
		}
		if err := ValidateConsultationSlotCapacity(ctx, tx, slotID, slot.Capacity); err != nil {
			return err
		}
		return tx.Model(&current).Select("owner_id", "date", "time_start", "time_end", "capacity", "is_available").Updates(slot).Error
	})
}

func parseConsultationDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("date is required")
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func ConsultationRescheduleLimit(ctx context.Context, db *gorm.DB) int {
	var setting models.Setting
	if err := db.WithContext(ctx).Where("`key` = ?", ConsultationRescheduleLimitKey).First(&setting).Error; err != nil {
		return 1
	}
	limit, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil || limit < 0 {
		return 1
	}
	if limit > 10 {
		return 10
	}
	return limit
}

func ListConsultationSlots(ctx context.Context, db *gorm.DB, ownerID *uint) ([]ConsultationSlotRow, error) {
	rows := make([]ConsultationSlotRow, 0)
	query := db.WithContext(ctx).Table("consultation_slots cs").
		Select(`
			cs.id, cs.owner_id, COALESCE(owner.name, 'Academy team') AS owner_name,
			cs.date, cs.time_start, cs.time_end, cs.capacity, cs.is_available,
			COUNT(CASE WHEN cb.status IN ('confirmed', 'pending') THEN cb.id END) AS active_bookings,
			GREATEST(cs.capacity - COUNT(CASE WHEN cb.status IN ('confirmed', 'pending') THEN cb.id END), 0) AS available_capacity,
			COUNT(CASE WHEN cb.status IN ('confirmed', 'pending') THEN cb.id END) >= cs.capacity AS is_booked
		`).
		Joins("LEFT JOIN users owner ON owner.id = cs.owner_id").
		Joins("LEFT JOIN consultation_bookings cb ON cb.slot_id = cs.id").
		Where("cs.date >= CURDATE()")
	if ownerID != nil {
		query = query.Where("cs.owner_id = ?", *ownerID)
	}
	err := query.
		Group("cs.id, cs.owner_id, owner.name, cs.date, cs.time_start, cs.time_end, cs.capacity, cs.is_available").
		Order("cs.date ASC, cs.time_start ASC").
		Scan(&rows).Error
	return rows, err
}

func ConsultationBookingRows(ctx context.Context, db *gorm.DB, userID uint) ([]ConsultationBookingRow, error) {
	return ConsultationBookingRowsForOwner(ctx, db, userID, nil)
}

func ConsultationBookingRowsForOwner(ctx context.Context, db *gorm.DB, userID uint, ownerID *uint) ([]ConsultationBookingRow, error) {
	query := db.WithContext(ctx).Table("consultation_bookings cb").
		Select(`cb.id, cb.slot_id, cs.owner_id, COALESCE(owner.name, 'Academy team') AS owner_name, cb.user_id, u.name AS user_name, u.email AS user_email, cb.order_id, COALESCE(p.title_en, c.title_en, 'Consultation') AS product, cs.date, cs.time_start, cs.time_end, cb.status, cb.notes, cb.reschedule_count, cb.created_at`).
		Joins("JOIN consultation_slots cs ON cs.id = cb.slot_id").
		Joins("LEFT JOIN users owner ON owner.id = cs.owner_id").
		Joins("JOIN users u ON u.id = cb.user_id").
		Joins("JOIN orders o ON o.id = cb.order_id").
		Joins("LEFT JOIN order_items oi ON oi.order_id = o.id").
		Joins("LEFT JOIN products p ON p.id = oi.item_id AND oi.item_type = 'product'").
		Joins("LEFT JOIN courses c ON c.id = oi.item_id AND oi.item_type = 'course'").
		Order("cs.date desc, cs.time_start desc")
	if userID != 0 {
		query = query.Where("cb.user_id = ?", userID)
	}
	if ownerID != nil {
		query = query.Where("cs.owner_id = ?", *ownerID)
	}
	rows := make([]ConsultationBookingRow, 0)
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ApplyConsultationRescheduleLimit(rows []ConsultationBookingRow, limit int) {
	for index := range rows {
		rows[index].RescheduleLimit = limit
	}
}

func EnsureConsultationOrder(ctx context.Context, db *gorm.DB, userID uint, orderID uint) error {
	_, err := ConsultationRequirementForOrder(ctx, db, userID, orderID)
	return err
}

func ConsultationRequirementForOrder(ctx context.Context, db *gorm.DB, userID uint, orderID uint) (ConsultationBookingRequirement, error) {
	var rows []struct {
		OwnerID uint
		Service string
	}
	if err := db.WithContext(ctx).Raw(`
		SELECT owner_id, service FROM (
			SELECT 0 AS owner_id, COALESCE(p.title_en, 'Consultation') AS service, 1 AS priority
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id AND oi.item_type = 'product'
			JOIN products p ON p.id = oi.item_id
			JOIN product_categories pc ON pc.id = p.category_id
			WHERE o.id = ? AND o.user_id = ? AND o.status = 'success' AND pc.requires_booking_time = true

			UNION ALL

			SELECT COALESCE(c.instructor_id, 0) AS owner_id, COALESCE(c.title_en, 'Course consultation') AS service, 0 AS priority
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id AND oi.item_type = 'course'
			JOIN courses c ON c.id = oi.item_id
			JOIN course_addons ca ON ca.course_id = c.id AND ca.is_active = true
			JOIN product_categories pc ON pc.id = ca.product_category_id
			WHERE o.id = ? AND o.user_id = ? AND o.status = 'success' AND pc.requires_booking_time = true
		) requirement
		ORDER BY priority ASC
		LIMIT 1
	`, orderID, userID, orderID, userID).Scan(&rows).Error; err != nil {
		return ConsultationBookingRequirement{}, fmt.Errorf("Failed to validate order")
	}
	if len(rows) == 0 {
		return ConsultationBookingRequirement{}, fmt.Errorf("A successful booking-time product purchase is required before booking")
	}
	return ConsultationBookingRequirement{OwnerID: rows[0].OwnerID, Service: rows[0].Service}, nil
}

func NotifyAdminsAboutBooking(ctx context.Context, db *gorm.DB, user models.User, slot models.ConsultationSlot, booking models.ConsultationBooking) error {
	var admins []models.User
	if err := db.WithContext(ctx).Where("is_admin = ?", true).Find(&admins).Error; err != nil {
		return err
	}
	if len(admins) == 0 && slot.OwnerID == 0 {
		return nil
	}
	type recipient struct {
		userID uint
		link   string
	}
	recipients := make(map[uint]recipient)
	adminLink := fmt.Sprintf("/admin/consultations?booking=%d", booking.ID)
	for _, admin := range admins {
		recipients[admin.ID] = recipient{userID: admin.ID, link: adminLink}
	}
	if slot.OwnerID != 0 {
		if _, exists := recipients[slot.OwnerID]; !exists {
			recipients[slot.OwnerID] = recipient{userID: slot.OwnerID, link: fmt.Sprintf("/instructor/consultations?booking=%d", booking.ID)}
		}
	}
	notifications := make([]models.Notification, 0, len(recipients))
	dateLabel := slot.Date.Format("2006-01-02") + " " + slot.TimeStart + "-" + slot.TimeEnd
	for _, recipient := range recipients {
		notifications = append(notifications, models.Notification{
			UserID:    recipient.userID,
			TitleEn:   "New consultation booking",
			TitleID:   "Booking konsultasi baru",
			MessageEn: user.Name + " booked a consultation slot on " + dateLabel + ".",
			MessageID: user.Name + " memilih jadwal konsultasi pada " + dateLabel + ".",
			Type:      "consultation_booking",
			Link:      recipient.link,
		})
	}
	return db.WithContext(ctx).Create(&notifications).Error
}

func BookConsultationSlot(ctx context.Context, db *gorm.DB, cfg config.Config, user models.User, slotID uint, orderID uint, notes string) (models.ConsultationBooking, error) {
	requirement, err := ConsultationRequirementForOrder(ctx, db, user.ID, orderID)
	if err != nil {
		return models.ConsultationBooking{}, err
	}
	var booking models.ConsultationBooking
	var bookedSlot models.ConsultationSlot
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND status = ?", orderID, user.ID, "success").First(&order).Error; err != nil {
			return fmt.Errorf("A successful booking-time product purchase is required before booking")
		}
		var slot models.ConsultationSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, slotID).Error; err != nil {
			return fmt.Errorf("Slot not found")
		}
		if slot.OwnerID != requirement.OwnerID {
			return fmt.Errorf("Choose a slot from the correct instructor or service provider")
		}
		if !slot.IsAvailable {
			return fmt.Errorf("Slot is not available")
		}
		var count int64
		if err := tx.Model(&models.ConsultationBooking{}).Where("slot_id = ? AND status IN ?", slot.ID, []string{"pending", "confirmed"}).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(slot.Capacity) {
			return fmt.Errorf("Slot capacity is full")
		}
		if err := tx.Model(&models.ConsultationBooking{}).Where("order_id = ? AND user_id = ?", orderID, user.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("This order already has a booking history")
		}
		booking = models.ConsultationBooking{SlotID: slot.ID, UserID: user.ID, OrderID: orderID, Status: "confirmed", Notes: notes}
		if err := tx.Create(&booking).Error; err != nil {
			return err
		}
		bookedSlot = slot
		return NotifyAdminsAboutBooking(ctx, tx, user, slot, booking)
	})
	if err == nil {
		_ = SendTransactionalTemplateEmail(ctx, db, EmailTemplateBookingConfirmation, "booking_confirmation", user, map[string]string{
			"booking_time":  bookedSlot.Date.Format("2006-01-02") + " " + bookedSlot.TimeStart + "-" + bookedSlot.TimeEnd,
			"service":       requirement.Service,
			"dashboard_url": localizedFrontendURL(cfg, user.Language, "/dashboard/bookings"),
		})
	}
	return booking, err
}

func UpdateConsultationBooking(ctx context.Context, db *gorm.DB, userID uint, bookingID uint, slotID uint, status string, notes string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking models.ConsultationBooking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", bookingID, userID).First(&booking).Error; err != nil {
			return fmt.Errorf("Booking not found")
		}
		if booking.Status == "cancelled" || booking.Status == "completed" {
			return fmt.Errorf("Booking cannot be changed")
		}
		var currentSlot models.ConsultationSlot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&currentSlot, booking.SlotID).Error; err != nil {
			return fmt.Errorf("Current slot not found")
		}
		if status == "cancelled" {
			if err := tx.Model(&booking).Updates(map[string]any{"status": "cancelled", "notes": notes}).Error; err != nil {
				return err
			}
			return nil
		}
		if slotID != 0 && slotID != booking.SlotID {
			limit := ConsultationRescheduleLimit(ctx, tx)
			if booking.RescheduleCount >= limit {
				return fmt.Errorf("Reschedule limit reached")
			}
			var nextSlot models.ConsultationSlot
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&nextSlot, slotID).Error; err != nil {
				return fmt.Errorf("Slot not found")
			}
			if nextSlot.OwnerID != currentSlot.OwnerID {
				return fmt.Errorf("Choose a slot from the same instructor or service provider")
			}
			if !nextSlot.IsAvailable {
				return fmt.Errorf("Slot is not available")
			}
			var activeBookings int64
			if err := tx.Model(&models.ConsultationBooking{}).Where("slot_id = ? AND status IN ?", nextSlot.ID, []string{"pending", "confirmed"}).Count(&activeBookings).Error; err != nil {
				return err
			}
			if activeBookings >= int64(nextSlot.Capacity) {
				return fmt.Errorf("Slot capacity is full")
			}
			return tx.Model(&booking).Updates(map[string]any{
				"slot_id":          nextSlot.ID,
				"status":           "confirmed",
				"notes":            notes,
				"reschedule_count": gorm.Expr("reschedule_count + ?", 1),
			}).Error
		}
		return tx.Model(&booking).Update("notes", notes).Error
	})
}

func DeleteConsultationSlot(ctx context.Context, db *gorm.DB, slotID uint, ownerID *uint) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var slot models.ConsultationSlot
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", slotID)
		if ownerID != nil {
			query = query.Where("owner_id = ?", *ownerID)
		}
		if err := query.First(&slot).Error; err != nil {
			return fmt.Errorf("Slot not found")
		}
		var bookingCount int64
		if err := tx.Model(&models.ConsultationBooking{}).Where("slot_id = ?", slot.ID).Count(&bookingCount).Error; err != nil {
			return err
		}
		if bookingCount > 0 {
			return fmt.Errorf("A slot with booking history cannot be deleted")
		}
		return tx.Delete(&slot).Error
	})
}

func UpdateConsultationBookingByProvider(ctx context.Context, db *gorm.DB, bookingID uint, ownerID *uint, status string, notes string) error {
	allowed := map[string]bool{"": true, "pending": true, "confirmed": true, "completed": true, "cancelled": true, "no_show": true}
	status = strings.TrimSpace(status)
	if !allowed[status] {
		return fmt.Errorf("Invalid booking status")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking models.ConsultationBooking
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", bookingID)
		if ownerID != nil {
			query = query.Where("slot_id IN (SELECT id FROM consultation_slots WHERE owner_id = ?)", *ownerID)
		}
		if err := query.First(&booking).Error; err != nil {
			return fmt.Errorf("Booking not found")
		}
		updates := map[string]any{"notes": notes}
		if status != "" && status != booking.Status {
			if status == "pending" || status == "confirmed" {
				var slot models.ConsultationSlot
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, booking.SlotID).Error; err != nil {
					return fmt.Errorf("Slot not found")
				}
				if !slot.IsAvailable {
					return fmt.Errorf("Slot is not available")
				}
				var activeBookings int64
				if err := tx.Model(&models.ConsultationBooking{}).
					Where("slot_id = ? AND id <> ? AND status IN ?", slot.ID, booking.ID, []string{"pending", "confirmed"}).
					Count(&activeBookings).Error; err != nil {
					return err
				}
				if activeBookings >= int64(slot.Capacity) {
					return fmt.Errorf("Slot capacity is full")
				}
			}
			updates["status"] = status
		}
		return tx.Model(&booking).Updates(updates).Error
	})
}
