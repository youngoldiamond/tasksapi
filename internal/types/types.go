package types

type Task struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	Project string `json:"project"`
	Context string `json:"context"`
	Done    bool   `json:"done"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	ID int64
	Credentials
}
