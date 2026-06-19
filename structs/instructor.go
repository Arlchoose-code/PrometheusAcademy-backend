package structs

type InstructorCourseSummary struct {
	ID            uint   `json:"id"`
	TitleEn       string `json:"title_en"`
	TitleID       string `json:"title_id"`
	Slug          string `json:"slug"`
	Thumbnail     string `json:"thumbnail"`
	Status        string `json:"status"`
	Students      int64  `json:"students"`
	Conversations int64  `json:"conversations"`
	Unread        int64  `json:"unread"`
}

type InstructorStats struct {
	AssignedCourses   int64 `json:"assigned_courses"`
	Students          int64 `json:"students"`
	OpenConversations int64 `json:"open_conversations"`
	UnreadMessages    int64 `json:"unread_messages"`
}

type InstructorOverviewResponse struct {
	Stats   InstructorStats           `json:"stats"`
	Courses []InstructorCourseSummary `json:"courses"`
}
