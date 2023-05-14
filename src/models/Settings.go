package models

const (
	PageEncoding_Windows1251 = "cp1251"
	PageEncoding_UTF8        = "utf8"
)

type Settings struct {
	Database                *DatabaseSettings `json:"database"`
	TemporaryFolder         string            `json:"temporaryFolder"`
	ForumsFile              string            `json:"forumsFile"`
	PageEncoding            string            `json:"pageEncoding"`
	ForumTopicsPageDelaySec float64           `json:"forumTopicsPageDelaySec"`
	TopicsPerPage           uint              `json:"topicsPerPage"`
	UserAgent               string            `json:"userAgent"`
	Cookie                  string            `json:"cookie"`
	ForumUrlFormat          string            `json:"forumUrlFormat"`
	ArchivedTopicsForumId   uint              `json:"archivedTopicsForumId"`
}

type DatabaseSettings struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Db       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`

	// TemporaryFolder is taken from App's settings.
	TemporaryFolder string `json:"-"`
}
