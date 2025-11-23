package entity

type Balance struct {
	CustomerId    int `gorm:"primary_key"`
	BalanceBigint int64
}

func (Balance) TableName() string {
	return "balances"
}
