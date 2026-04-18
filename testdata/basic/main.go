package main

import "testbasic/order"

func main() {
	result, err := order.ProcessOrder(order.Order{
		ID:     1,
		Amount: 150.0,
		Items:  []order.Item{{Name: "Pizza", Price: 100.0}, {Name: "Coke", Price: 50.0}},
	})
	_ = result
	_ = err
}
