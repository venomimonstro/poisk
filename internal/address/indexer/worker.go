package indexer

import (
	"context"
	"errors"
	"fmt"
	"time"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
)

type AddressSource interface{Load(context.Context,int64,int64)(indexmanticore.AddressDocument,error)}
type AddressIndex interface{ApplyAddress(context.Context,indexmanticore.AddressDocument)(bool,error);DeleteAddress(context.Context,int64,int64)error}
type EventAcker interface{MarkProcessed(context.Context,int64,string)error;Retry(context.Context,int64,string,string,time.Duration)error}

type Processor struct{Source AddressSource;Index AddressIndex;Ack EventAcker;RetryBase,RetryMax time.Duration}

func (p Processor) Process(ctx context.Context,event outbox.Event)error{
	if p.Source==nil||p.Index==nil||p.Ack==nil{return errors.New("address index processor is not initialized")}
	if event.EntityType!="ADDRESS"{return fmt.Errorf("unexpected entity type %q",event.EntityType)}
	switch event.Operation{
	case "DELETE":
		if err:=p.Index.DeleteAddress(ctx,event.EntityID,event.EntityVersion);err!=nil{return p.retry(ctx,event,err)}
		return p.Ack.MarkProcessed(ctx,event.ID,event.WorkerID)
	case "UPSERT":
		doc,err:=p.Source.Load(ctx,event.EntityID,event.EntityVersion)
		if errors.Is(err,ErrAddressVersionMismatch){return p.Ack.MarkProcessed(ctx,event.ID,event.WorkerID)}
		if errors.Is(err,ErrAddressNotFound){return p.retry(ctx,event,err)}
		if err!=nil{return p.retry(ctx,event,err)}
		if doc.Status!="ACTIVE"{if err:=p.Index.DeleteAddress(ctx,event.EntityID,event.EntityVersion);err!=nil{return p.retry(ctx,event,err)};return p.Ack.MarkProcessed(ctx,event.ID,event.WorkerID)}
		if _,err:=p.Index.ApplyAddress(ctx,doc);err!=nil{return p.retry(ctx,event,err)}
		return p.Ack.MarkProcessed(ctx,event.ID,event.WorkerID)
	default:return p.retry(ctx,event,fmt.Errorf("unsupported address index operation %q",event.Operation))
	}
}
func (p Processor) retry(ctx context.Context,event outbox.Event,cause error)error{base:=p.RetryBase;if base<=0{base=time.Second};max:=p.RetryMax;if max<=0{max=time.Minute};delay:=base;for i:=int32(1);i<event.Attempts&&delay<max;i++{delay*=2;if delay>max{delay=max;break}};message:=cause.Error();if len(message)>1024{message=message[:1024]};if err:=p.Ack.Retry(ctx,event.ID,event.WorkerID,message,delay);err!=nil{return errors.Join(cause,err)};return nil}
