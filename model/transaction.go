package model

type Transaction struct {
	Id                int
	TransactionNumber int
	Name			  string
	Quantity          int
	Discount          float64
	Total             float64
	Pay				  float64
}

type TransactionDetail struct {
	Id          int
	Item        string
	Price       float64
	Quantity    int
	Total       float64
	Transaction Transaction
}

type RabbitMQData struct {
	RandomInteger 			int
	Name		  			string
	Quantity	  			int
	Total		  			float64
	Discount	  			float64
	Pay			  			float64
	ListTransactionDetail	[]TransactionDetail
}
