package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
)

type sourceFake struct{ doc indexmanticore.OrganizationDocument; err error; loads int }
func (s *sourceFake) Load(context.Context,int64,int64)(indexmanticore.OrganizationDocument,error){s.loads++;return s.doc,s.err}

type indexFake struct{ applies int; deletes int }
func (i *indexFake) ApplyOrganization(context.Context,indexmanticore.OrganizationDocument)(bool,error){i.applies++;return true,nil}
func (i *indexFake) DeleteOrganization(context.Context,int64,int64)error{i.deletes++;return nil}

type ackFake struct{ processed int; retries int }
func (a *ackFake) MarkProcessed(context.Context,int64,string)error{a.processed++;return nil}
func (a *ackFake) Retry(context.Context,int64,string,string,time.Duration)error{a.retries++;return nil}

func TestStaleOrganizationVersionIsAckedWithoutWrite(t *testing.T){
	source:=&sourceFake{err:fmtVersionMismatch()};index:=&indexFake{};ack:=&ackFake{}
	processor:=Processor{Source:source,Index:index,Ack:ack}
	event:=outbox.Event{ID:1,EntityType:"ORGANIZATION",EntityID:9,EntityVersion:1,Operation:"UPSERT",WorkerID:"w",Attempts:1}
	if err:=processor.Process(context.Background(),event);err!=nil{t.Fatal(err)}
	if ack.processed!=1||index.applies!=0||ack.retries!=0{t.Fatalf("processed=%d applies=%d retries=%d",ack.processed,index.applies,ack.retries)}
}

func TestOrganizationProcessorRejectsOtherEntityTypes(t *testing.T){
	processor:=Processor{Source:&sourceFake{},Index:&indexFake{},Ack:&ackFake{}}
	err:=processor.Process(context.Background(),outbox.Event{EntityType:"WEB_DOCUMENT"})
	if err==nil{t.Fatal("expected entity type error")}
}

func TestInactiveOrganizationDeletesIndexDocument(t *testing.T){
	source:=&sourceFake{doc:indexmanticore.OrganizationDocument{ID:4,EntityVersion:2,Name:"X",NormalizedName:"x",Status:"RETIRED"}}
	index:=&indexFake{};ack:=&ackFake{}
	processor:=Processor{Source:source,Index:index,Ack:ack}
	event:=outbox.Event{ID:2,EntityType:"ORGANIZATION",EntityID:4,EntityVersion:2,Operation:"UPSERT",WorkerID:"w",Attempts:1}
	if err:=processor.Process(context.Background(),event);err!=nil{t.Fatal(err)}
	if index.deletes!=1||ack.processed!=1{t.Fatalf("deletes=%d processed=%d",index.deletes,ack.processed)}
}

func fmtVersionMismatch()error{return errors.Join(ErrOrganizationVersionMismatch,errors.New("newer canonical version"))}
