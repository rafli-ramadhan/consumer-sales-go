package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
	amqp "github.com/rabbitmq/amqp091-go"

	"consumer-sales-go/db"
	"consumer-sales-go/model"
	"consumer-sales-go/helpers/random"
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

type TransactionData struct {
	Voucher 				model.VoucherRequest
	ListTransactionDetail 	[]model.TransactionDetail
	Req 					model.TransactionDetailBulkRequest
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"create_transaction", // name
		true,                 // durable
		false,                // auto delete queue when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	FailOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	FailOnError(err, "Failed to register a consumer")

	// consumer must always be on and the channel to prevent the consumer from turning off
	var forever chan string

	// worker to receive value from variable msgs
	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)

			var data TransactionData
			err := json.Unmarshal(d.Body, &data)
			if err != nil {
				FailOnError(err, "error unmarshal")
			}

			_, err = CreateBulkTransactionDetail(data.Voucher, data.ListTransactionDetail, data.Req)
			if err != nil {
				log.Print(err)
			}

			d.Ack(false)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")

	// channel in to prevent consumer to turning off
	<-forever
}

func CreateBulkTransactionDetail(voucher model.VoucherRequest, listTransactionDetail []model.TransactionDetail, req model.TransactionDetailBulkRequest) (res []model.TransactionDetail, err error) {
	// sum all quantity and total
	var quantity int
	var total float64
	for _, item := range listTransactionDetail {
		quantity = quantity + item.Quantity
		total = total + item.Total
	}

	// discount calculation
	var discount float64
	if total > 300000 && voucher.Persen > 0 {
		discount = voucher.Persen/100
		total = total*(1-discount)
	}

	if req.Pay < total {
		err = errors.New("pay must be > total")
		return
	}

	// generate random integer
	randomInteger, err := random.RandomString(9)
    if err != nil {
        return
    }
	log.Print(randomInteger)
	
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

	log.Print("PASS 1")
	var lastIDTransaction int32
	err = stmt.QueryRowContext(ctx, randomInteger, req.Name, quantity, discount, total, req.Pay).Scan(&lastIDTransaction)
	if err != nil {
		trx.Rollback()
		return
	}

	log.Print("PASS 2")
	query2 := `INSERT INTO transaction_detail (transaction_id, item, price, quantity, total) values ($1, $2, $3, $4, $5) RETURNING id`
	stmt2, err := db.PrepareContext(ctx, query2)
	if err != nil {
		return
	}

	log.Print("PASS 3")
	for _, v := range listTransactionDetail {
		var lastID int32
		err := stmt2.QueryRowContext(ctx, lastIDTransaction, v.Item, v.Price, v.Quantity, v.Total).Scan(&lastID)
		if err != nil {
			trx.Rollback()
			return []model.TransactionDetail{}, err
		}

		newTransaction := model.TransactionDetail{
			Id:       int(lastID),
			Item:	  v.Item,
			Price:	  v.Price,
			Quantity: v.Quantity,
			Total:	  v.Total,
			Transaction: model.Transaction{
				Id: 	  		   int(lastIDTransaction),
				TransactionNumber: randomInteger,
				Name:	  		   req.Name,
				Quantity: 		   quantity,
				Discount: 		   discount,
				Total:	  		   total,
				Pay:	  		   req.Pay,
			},
		}
		res = append(res, newTransaction)
	}

	trx.Commit()

	return
}
