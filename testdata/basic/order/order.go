package order

func ProcessOrder(o Order) (OrderResult, error) {
	total := calculateTotal(o.Items)

	var disc Discounter
	if total > 200 {
		disc = PercentDiscount{Percent: 10}
	} else if total > 100 {
		disc = FlatDiscount{Amount: 20}
	}

	discount := applyDiscount(total, disc)
	final := total - discount

	result := OrderResult{
		OrderID:  o.ID,
		Total:    total,
		Discount: discount,
		Final:    final,
	}

	return result, nil
}

func calculateTotal(items []Item) float64 {
	total := 0.0
	for _, item := range items {
		total = total + item.Price
	}
	return total
}

func applyDiscount(amount float64, d Discounter) float64 {
	if d == nil {
		return 0
	}
	return d.Calculate(amount)
}
