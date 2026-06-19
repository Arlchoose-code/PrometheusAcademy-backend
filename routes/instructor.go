package routes

import (
	"academyprometheus/backend/config"
	instructorcontroller "academyprometheus/backend/controllers/instructor"
	"academyprometheus/backend/middlewares"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterInstructorRoutes(router *gin.RouterGroup, db *gorm.DB, cfg config.Config) {
	uploadService := services.NewUploadService(db, cfg)
	controller := instructorcontroller.NewController(db, uploadService)
	instructor := router.Group("/instructor")
	instructor.Use(middlewares.AuthGuard(db, cfg), middlewares.RoleGuard("instructor"))

	instructor.GET("/overview", controller.GetOverview)
	instructor.GET("/courses", controller.ListCourses)
	instructor.POST("/courses", controller.CreateCourse)
	instructor.GET("/courses/:id", controller.GetCourse)
	instructor.PUT("/courses/:id", controller.UpdateCourse)
	instructor.DELETE("/courses/:id", controller.DeleteCourse)
	instructor.POST("/courses/:id/thumbnail", controller.UpdateCourseThumbnail)
	instructor.GET("/consultations/slots", controller.ListConsultationSlots)
	instructor.POST("/consultations/slots", controller.CreateConsultationSlot)
	instructor.PUT("/consultations/slots/:id", controller.UpdateConsultationSlot)
	instructor.DELETE("/consultations/slots/:id", controller.DeleteConsultationSlot)
	instructor.GET("/consultations/bookings", controller.ListConsultationBookings)
	instructor.PUT("/consultations/bookings/:id", controller.UpdateConsultationBooking)
	instructor.POST("/courses/:id/modules", controller.CreateCourseModule)
	instructor.POST("/courses/:id/add-ons", controller.CreateCourseAddon)
	instructor.PUT("/course-addons/:id", controller.UpdateCourseAddon)
	instructor.POST("/course-addons/:id/file", controller.UpdateCourseAddonFile)
	instructor.DELETE("/course-addons/:id", controller.DeleteCourseAddon)
	instructor.PUT("/modules/:id", controller.UpdateCourseModule)
	instructor.DELETE("/modules/:id", controller.DeleteCourseModule)
	instructor.PUT("/modules/reorder", controller.ReorderCourseModules)
	instructor.POST("/modules/:id/topics", controller.CreateTopic)
	instructor.POST("/modules/:id/quizzes", controller.CreateQuiz)
	instructor.PUT("/topics/:id", controller.UpdateTopic)
	instructor.DELETE("/topics/:id", controller.DeleteTopic)
	instructor.PUT("/topics/reorder", controller.ReorderTopics)
	instructor.POST("/topics/:id/blocks", controller.CreateTopicBlock)
	instructor.POST("/topics/:id/assignments", controller.CreateAssignment)
	instructor.PUT("/topic-blocks/:id", controller.UpdateTopicBlock)
	instructor.DELETE("/topic-blocks/:id", controller.DeleteTopicBlock)
	instructor.PUT("/topic-blocks/reorder", controller.ReorderTopicBlocks)
	instructor.POST("/topic-blocks/:id/file", controller.UpdateTopicBlockFile)
	instructor.PUT("/assignments/:id", controller.UpdateAssignment)
	instructor.DELETE("/assignments/:id", controller.DeleteAssignment)
	instructor.PUT("/quizzes/:id", controller.UpdateQuiz)
	instructor.DELETE("/quizzes/:id", controller.DeleteQuiz)
	instructor.PUT("/quizzes/reorder", controller.ReorderQuizzes)
	instructor.POST("/quizzes/:id/questions", controller.CreateQuizQuestion)
	instructor.PUT("/quiz-questions/:id", controller.UpdateQuizQuestion)
	instructor.DELETE("/quiz-questions/:id", controller.DeleteQuizQuestion)
	instructor.POST("/quiz-questions/:id/answers", controller.CreateQuizAnswer)
	instructor.PUT("/quiz-answers/:id", controller.UpdateQuizAnswer)
	instructor.DELETE("/quiz-answers/:id", controller.DeleteQuizAnswer)
	instructor.GET("/notifications", controller.ListNotifications)
	instructor.POST("/notifications/mark-all-read", controller.MarkAllNotificationsRead)
	instructor.GET("/communications", controller.ListCommunications)
	instructor.GET("/communications/:id/messages", controller.ListCommunicationMessages)
	instructor.POST("/communications/:id/messages", controller.ReplyCommunication)
	instructor.PATCH("/communications/:id", controller.UpdateCommunicationStatus)
}
