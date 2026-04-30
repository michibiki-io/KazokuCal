package calendar

type Color string

const (
	ColorBlack Color = "black"
	ColorRed   Color = "red"
	ColorBlue  Color = "blue"
)

type WeekStart string

const (
	WeekStartSunday WeekStart = "sunday"
	WeekStartMonday WeekStart = "monday"
)

type TeleworkStatus struct {
	Papa bool `json:"papa"`
	Mama bool `json:"mama"`
}

type ScheduleScope string

const (
	ScheduleScopePersonal ScheduleScope = "personal"
	ScheduleScopeGroup    ScheduleScope = "group"
	ScheduleScopeWorld    ScheduleScope = "world"
)

type ScheduleItem struct {
	ID        string        `json:"id"`
	SourceID  string        `json:"sourceId,omitempty"`
	Date      string        `json:"date"`
	Text      string        `json:"text"`
	Color     Color         `json:"color"`
	ScopeType ScheduleScope `json:"scopeType,omitempty"`
	Group     string        `json:"group,omitempty"`
}

type MultiDayScheduleItem struct {
	ID        string        `json:"id"`
	SourceID  string        `json:"sourceId,omitempty"`
	StartDate string        `json:"startDate"`
	EndDate   string        `json:"endDate"`
	Text      string        `json:"text"`
	Color     Color         `json:"color"`
	Arrow     bool          `json:"arrow"`
	ScopeType ScheduleScope `json:"scopeType,omitempty"`
	Group     string        `json:"group,omitempty"`
}

type PDFRequest struct {
	Year          int                       `json:"year"`
	Month         int                       `json:"month"`
	WeekStartsOn  WeekStart                 `json:"weekStartsOn"`
	Telework      map[string]TeleworkStatus `json:"telework"`
	ScheduleItems []ScheduleItem            `json:"scheduleItems"`
	MultiDayItems []MultiDayScheduleItem    `json:"multiDayItems"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
