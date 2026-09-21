package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/billing"
)

func runBillingCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: billingctl payment|reconcile")};repo:=billing.NewRepository(pool)
	switch args[0]{
	case "payment":
		if len(args)!=6{return errors.New("usage: billingctl payment <provider> <event-id> <type> <invoice-id> <amount-kopecks>; raw provider payload on stdin")}
		invoiceID,err:=strconv.ParseInt(args[4],10,64);if err!=nil||invoiceID<=0{return errors.New("invalid invoice id")};amount,err:=strconv.ParseInt(args[5],10,64);if err!=nil||amount<0{return errors.New("invalid amount")}
		payload,err:=io.ReadAll(io.LimitReader(os.Stdin,64<<10));if err!=nil{return err};if len(payload)>64<<10{return errors.New("provider payload exceeds 64 KiB")}
		result,err:=repo.ApplyPaymentEvent(ctx,billing.PaymentEvent{Provider:args[1],ProviderEventID:args[2],Type:args[3],InvoiceID:invoiceID,AmountKopecks:amount,Currency:"RUB",Payload:payload,OccurredAt:time.Now().UTC()});if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(result)
	case "reconcile":
		graced,expired,err:=repo.ReconcileLifecycle(ctx,time.Now().UTC());if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(map[string]int64{"graced":graced,"expired":expired})
	default:return fmt.Errorf("unknown billingctl command %q",args[0])
	}
}
