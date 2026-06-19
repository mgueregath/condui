package transfers

type TransferTask struct {
	ID string `json:"id"`

	FileName string `json:"fileName"`

	Direction string `json:"direction"`

	Progress int64 `json:"progress"`

	Transferred int64 `json:"transferred"`

	Total int64 `json:"total"`

	Status string `json:"status"`
}
