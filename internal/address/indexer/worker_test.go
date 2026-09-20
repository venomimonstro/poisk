package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
)

type fakeIndex struct{applied int;deleted int}
func (f *fakeIndex) ApplyAddress(context.Context,indexmanticore.AddressDocument)(bool,error){f.applied++;return true,nil}
func (f *fakeIndex) DeleteAddress(context.Context,int64,int64)error{f.deleted++;return nil}
type fakeAck struct{processed int;retried int}
func (f *fakeAck) MarkProcessed(context.Context,int64,string)error{f.processed++;return nil}
func (f *fakeAck) Retry(context.Context,int64,string,string,time.Duration)error{f.retried++;return nil}
type loadFunc func(context.Context,int64,int64)(indexmanticore.AddressDocument,error)
func (f loadFunc) Load(ctx context.Context,id,version int64)(indexmanticore.AddressDocument,error){return f(ctx,id,version)}

func TestProcessorRejectsForeignEntity(t *testing.T){
	p:=Processor{Source:loadFunc(func(context.Context,int64,int64)(indexmanticore.AddressDocument,error){return indexmanticore.AddressDocument{},nil}),Index:&fakeIndex{},Ack:&fakeAck{}}
	err:=p.Process(context.Background(),outbox.Event{EntityType:"WEB_DOCUMENT"})
	if err==nil{t.Fatal("expected error")}
}

func TestProcessorStaleVersionIsAcknowledgedWithoutWrite(t *testing.T){
	index:=&fakeIndex{};ack:=&fakeAck{}
	source:=loadFunc(func(context.Context,int64,int64)(indexmanticore.AddressDocument,error){return indexmanticore.AddressDocument{},ErrAddressVersionMismatch})
	p:=Processor{Source:source,Index:index,Ack:ack}
	event:=outbox.Event{ID:7,EntityType:"ADDRESS",EntityID:11,EntityVersion:2,Operation:"UPSERT",WorkerID:"w"}
	if err:=p.Process(context.Background(),event);err!=nil{t.Fatal(err)}
	if index.applied!=0||index.deleted!=0||ack.processed!=1||ack.retried!=0{t.Fatalf("unexpected side effects index=%+v ack=%+v",index,ack)}
}

func TestRetryCapsDiagnostic(t *testing.T){
	ack:=&fakeAck{};p:=Processor{Ack:ack,RetryBase:time.Millisecond,RetryMax:2*time.Millisecond}
	event:=outbox.Event{ID:1,WorkerID:"w",Attempts:4}
	if err:=p.retry(context.Background(),event,errors.New("temporary"));err!=nil{t.Fatal(err)}
	if ack.retried!=1{t.Fatalf("retried=%d",ack.retried)}
}
