package domain

type UserState struct {
	State      string
	Count      int
	IsPaid     bool
	ShampooCnt int
	PerfumeCnt int
	Contact    string
	Address    string
	PostIndex  string
	QRs        []string // 👈 список всех QR по пользователю
}
