package organizations

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeSourceRowRussianPhoneAndIDNWebsite(t *testing.T){
	lat,lon:=55.75,37.62
	row:=SourceRow{SourceRecordID:" 42 ",Name:"  Клиника   Слуха  ",Phone:"8 (999) 123-45-67",Website:"пример.рф/catalog/",Address:" Москва,  Тверская 1 ",CategoryKey:"medical",Latitude:&lat,Longitude:&lon}
	got,err:=NormalizeSourceRow(row);if err!=nil{t.Fatal(err)}
	if got.SourceRecordID!="42"||got.NormalizedName!="клиника слуха"||got.Phone!="+79991234567"{t.Fatalf("got=%+v",got)}
	if !strings.HasPrefix(got.Website,"https://xn--")||got.NormalizedAddress!="москва тверская 1"{t.Fatalf("got=%+v",got)}
	var raw SourceRow
	if err:=json.Unmarshal(got.RawPayload,&raw);err!=nil{t.Fatal(err)}
	if raw.SourceRecordID!=" 42 "{t.Fatalf("raw payload was normalized instead of preserved: %q",raw.SourceRecordID)}
	if len(got.PayloadHash)!=64{t.Fatalf("hash=%q",got.PayloadHash)}
}

func TestNormalizeSourceRowRejectsInvalidCoordinatesAndPhone(t *testing.T){
	lat:=95.0
	if _,err:=NormalizeSourceRow(SourceRow{SourceRecordID:"1",Name:"X",Latitude:&lat});!errors.Is(err,ErrInvalidRow){t.Fatalf("coords err=%v",err)}
	if _,err:=NormalizeSourceRow(SourceRow{SourceRecordID:"1",Name:"X",Phone:"123"});!errors.Is(err,ErrInvalidRow){t.Fatalf("phone err=%v",err)}
}

func TestNormalizeSourceRowHashIgnoresCosmeticWhitespace(t *testing.T){
	a,err:=NormalizeSourceRow(SourceRow{SourceRecordID:"1",Name:"ООО  Ромашка",Address:"ул. Ленина  1"});if err!=nil{t.Fatal(err)}
	b,err:=NormalizeSourceRow(SourceRow{SourceRecordID:"1",Name:"ООО Ромашка",Address:"ул. Ленина 1"});if err!=nil{t.Fatal(err)}
	if a.PayloadHash!=b.PayloadHash{t.Fatalf("cosmetic whitespace changed normalized hash: %s %s",a.PayloadHash,b.PayloadHash)}
}
