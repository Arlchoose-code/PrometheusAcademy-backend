package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

const ConsultationRescheduleLimitKey = "consultation_reschedule_limit"

type ConsultationSlotRow struct {
	ID          uint      `json:"id"`
	Date        time.Time `json:"date"`
	TimeStart   string    `json:"time_start"`
	TimeEnd     string    `json:"time_end"`
	IsAvailable bool      `json:"is_available"`
	IsBooked    bool      `json:"is_booked"`
}

type ConsultationBookingRow struct {
	ID              uint      `json:"id"`
	SlotID          uint      `json:"slot_id"`
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

func ListConsultationSlots(ctx context.Context, db *gorm.DB) ([]ConsultationSlotRow, error) {
	var rows []ConsultationSlotRow
	err := db.WithContext(ctx).Raw(`
		SELECT cs.id, cs.date, cs.time_start, cs.time_end, cs.is_available,
			COUNT(CASE WHEN cb.status IN ('confirmed', 'pending') THEN cb.id END) > 0 AS is_booked
		FROM consultation_slots cs
		LEFT JOIN consultation_bookings cb ON cb.slot_id = cs.id
		WHERE cs.date >= CURDATE()
		GROUP BY cs.id, cs.date, cs.time_start, cs.time_end, cs.is_available
		ORDER BY cs.date ASC, cs.time_start ASC
	`).Scan(&rows).Error
	return rows, err
}

func ConsultationBookingRows(ctx context.Context, db *gorm.DB, userID uint) ([]ConsultationBookingRow, error) {
	query := db.WithContext(ctx).Table("consultation_bookings cb").
		Select(`cb.id, cb.slot_id, cb.user_id, u.name AS user_name, u.email AS user_email, cb.order_id, COALESCE(p.title_en, 'Consultation') AS product, cs.date, cs.time_start, cs.time_end, cb.status, cb.notes, cb.reschedule_count, cb.created_at`).
		Joins("JOIN consultation_slots cs ON cs.id = cb.slot_id").
		Joins("JOIN users u ON u.id = cb.user_id").
		Joins("JOIN orders o ON o.id = cb.order_id").
		Joins("LEFT JOIN order_items oi ON oi.order_id = o.id AND oi.item_type = 'product'").
		Joins("LEFT JOIN products p ON p.id = oi.item_id").
		Order("cs.date desc, cs.time_start desc")
	if userID != 0 {
		query = query.Where("cb.user_id = ?", userID)
	}
	var rows []ConsultationBookingRow
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
	var count int64
	if err := db.WithContext(ctx).Table("orders").
		Joins("JOIN order_items oi ON oi.order_id = orders.id").
		Joins("JOIN products p ON p.id = oi.item_id AND oi.item_type = 'product'").
		Joins("JOIN product_categories pc ON pc.id = p.category_id").
		Where("orders.id = ? AND orders.user_id = ? AND orders.status = ? AND pc.requires_booking_time = ?", orderID, userID, "success", true).
		Count(&count).Error; err != nil {
		return fmt.Errorf("Failed to validate order")
	}
	if count == 0 {
		return fmt.Errorf("A successful booking-time product purchase is required before booking")
	}
	return nil
}

func NotifyAdminsAboutBooking(ctx context.Context, db *gorm.DB, user models.User, slot models.ConsultationSlot, booking models.ConsultationBooking) error {
	var admins []models.User
	if err := db.WithContext(ctx).Where("is_admin = ?", true).Find(&admins).Error; err != nil {
		return err
	}
	if len(admins) == 0 {
		return nil
	}
	notifications := make([]models.Notification, 0, len(admins))
	dateLabel := slot.Date.Format("2006-01-02") + " " + slot.TimeStart + "-" + slot.TimeEnd
	for _, admin := range admins {
		notifications = append(notifications, models.Notification{
			UserID:    admin.ID,
			TitleEn:   "New consultation booking",
			TitleID:   "Booking konsultasi baru",
			MessageEn: user.Name + " booked a consultation slot on " + dateLabel + ".",
			MessageID: user.Name + " memilih jadwal konsultasi pada " + dateLabel + ".",
			Type:      "consultation_booking",
			Link:      fmt.Sprintf("/admin/consultations?booking=%d", booking.ID),
		})
	}
	return db.WithContext(ctx).Create(&notifications).Error
}

func BookConsultationSlot(ctx context.Context, db *gorm.DB, user models.User, slotID uint, orderID uint, notes string) (models.ConsultationBooking, error) {
	if err := EnsureConsultationOrder(ctx, db, user.ID, orderID); err != nil {
		return models.ConsultationBooking{}, err
	}
	var booking models.ConsultationBooking
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var slot models.ConsultationSlot
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&slot, slotID).Error; err != nil {
			return fmt.Errorf("Slot not found")
		}
		if !slot.IsAvailable {
			return fmt.Errorf("Slot is not available")
		}
		var count int64
		if err := tx.Model(&models.ConsultationBooking{}).Where("slot_id = ? AND status IN ?", slot.ID, []string{"pending", "confirmed"}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("Slot is already booked")
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
		if err := tx.Model(&slot).Update("is_available", false).Error; err != nil {
			return err
		}
		return NotifyAdminsAboutBooking(ctx, tx, user, slot, booking)
	})
	return booking, err
}

func UpdateConsultationBooking(ctx context.Context, db *gorm.DB, userID uint, bookingID uint, slotID uint, status string, notes string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking models.ConsultationBooking
		if err := tx.Where("id = ? AND user_id = ?", bookingID, userID).First(&booking).Error; err != nil {
			return fmt.Errorf("Booking not found")
		}
		if booking.Status == "cancelled" || booking.Status == "completed" {
			return fmt.Errorf("Booking cannot be changed")
		}
		if status == "cancelled" {
			if err := tx.Model(&booking).Updates(map[string]any{"status": "cancelled", "notes": notes}).Error; err != nil {
				return err
			}
			return tx.Model(&models.ConsultationSlot{}).Where("id = ?", booking.SlotID).Update("is_available", true).Error
		}
		if slotID != 0 && slotID != booking.SlotID {
			limit := ConsultationRescheduleLimit(ctx, tx)
			if booking.RescheduleCount >= limit {
				return fmt.Errorf("Reschedule limit reached")
			}
			var nextSlot models.ConsultationSlot
			if err := tx.First(&nextSlot, slotID).Error; err != nil {
				return fmt.Errorf("Slot not found")
			}
			if !nextSlot.IsAvailable {
				return fmt.Errorf("Slot is not available")
			}
			if err := tx.Model(&models.ConsultationSlot{}).Where("id = ?", booking.SlotID).Update("is_available", true).Error; err != nil {
				return err
			}
			if err := tx.Model(&nextSlot).Update("is_available", false).Error; err != nil {
				return err
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
