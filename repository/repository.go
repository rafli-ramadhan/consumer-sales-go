package repository

import (
	"context"
	"time"

	"consumer-sales-go/db"
	"consumer-sales-go/model"
)

func CreateBulkTransactionDetail(data model.RabbitMQData) (err error) {
	db := client.NewConnection(client.Database).GetMysqlConnection()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	trx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	query := `INSERT INTO transaction (transaction_number, name, quantity, discount, total, pay) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return
	}

	var lastIDTransaction int32
	err = stmt.QueryRowContext(ctx, data.RandomInteger, data.Name, data.Quantity, data.Discount, data.Total, data.Pay).Scan(&lastIDTransaction)
	if err != nil {
		trx.Rollback()
		return
	}

	query2 := `INSERT INTO transaction_detail (transaction_id, item, price, quantity, total) values ($1, $2, $3, $4, $5) RETURNING id`
	stmt2, err := db.PrepareContext(ctx, query2)
	if err != nil {
		return
	}

	for _, v := range data.ListTransactionDetail {
		var lastID int32
		err := stmt2.QueryRowContext(ctx, lastIDTransaction, v.Item, v.Price, v.Quantity, v.Total).Scan(&lastID)
		if err != nil {
			trx.Rollback()
			return err
		}
	}

	trx.Commit()

	return
}
