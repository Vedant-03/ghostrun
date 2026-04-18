package order

type Item struct {
	Name  string
	Price float64
}

type Order struct {
	ID     int
	Amount float64
	Items  []Item
}

type OrderResult struct {
	OrderID  int
	Total    float64
	Discount float64
	Final    float64
}

type Discounter interface {
	Calculate(amount float64) float64
}

type PercentDiscount struct {
	Percent float64
}

func (d PercentDiscount) Calculate(amount float64) float64 {
	return amount * d.Percent / 100.0
}

type FlatDiscount struct {
	Amount float64
}

func (d FlatDiscount) Calculate(amount float64) float64 {
	if amount > d.Amount {
		return d.Amount
	}
	return 0
}
