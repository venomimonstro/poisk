package organizations

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
)

type stageStoreFake struct{
	valid []NormalizedRow
	rejected []string
	finished int
}
func (s *stageStoreFake) StageValid(_ context.Context,_ int64,_ int64,row NormalizedRow)(int64,error){s.valid=append(s.valid,row);return int64(len(s.valid)),nil}
func (s *stageStoreFake) StageRejected(_ context.Context,_ int64,_ int64,_ string,_ []byte,code,_ string)(int64,error){s.rejected=append(s.rejected,code);return int64(len(s.rejected)),nil}
func (s *stageStoreFake) FinishStaging(context.Context,int64)error{s.finished++;return nil}

func TestStageJSONLMixesValidAndRejectedRows(t *testing.T){
	store:=&stageStoreFake{}
	input:=strings.Join([]string{
		`{"source_record_id":"1","name":"Аптека","phone":"89991234567"}`,
		`{"source_record_id":"2","name":""}`,
		`not-json`,
	},"\n")
	input=strings.ReplaceAll(input,`\"`,`"`)
	if err:=(Stager{Store:store}).StageJSONL(context.Background(),7,strings.NewReader(input));err!=nil{t.Fatal(err)}
	if len(store.valid)!=1{t.Fatalf("valid=%d",len(store.valid))}
	if len(store.rejected)!=2||store.rejected[0]!="INVALID_ROW"||store.rejected[1]!="INVALID_JSON"{t.Fatalf("rejected=%v",store.rejected)}
	if store.finished!=1{t.Fatalf("finished=%d",store.finished)}
}

func TestStageJSONLRejectsOversizedRowWithoutUnboundedBuffering(t *testing.T){
	store:=&stageStoreFake{}
	valid:=`{"source_record_id":"2","name":"После большой строки"}`
	valid=strings.ReplaceAll(valid,`\"`,`"`)
	input:=strings.Repeat("x",maxRawPayloadBytes+100)+"\n"+valid
	if err:=(Stager{Store:store}).StageJSONL(context.Background(),8,strings.NewReader(input));err!=nil{t.Fatal(err)}
	if len(store.rejected)!=1||store.rejected[0]!="ROW_TOO_LARGE"{t.Fatalf("rejected=%v",store.rejected)}
	if len(store.valid)!=1||store.valid[0].SourceRecordID!="2"{t.Fatalf("valid=%+v",store.valid)}
}

func TestReadBoundedLineHandlesFinalLineWithoutNewline(t *testing.T){
	buffer:=bufio.NewReader(strings.NewReader("abc"))
	line,tooLarge,err:=readBoundedLine(buffer,10)
	if string(line)!="abc"||tooLarge||err!=io.EOF{t.Fatalf("line=%q tooLarge=%v err=%v",line,tooLarge,err)}
}
