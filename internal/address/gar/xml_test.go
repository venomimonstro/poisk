package gar

import (
	"strings"
	"testing"
)

func TestAddressObjectStreamContinuesAfterInvalidRow(t *testing.T){
	xmlData:=`<OBJECTS>
<OBJECT OBJECTID="10" OBJECTGUID="550e8400-e29b-41d4-a716-446655440000" LEVEL="8" NAME="Тверская" TYPENAME="ул" ISACTUAL="1" ISACTIVE="1"/>
<OBJECT OBJECTID="bad" LEVEL="8" NAME="Плохая" TYPENAME="ул"/>
<OBJECT OBJECTID="12" LEVEL="6" NAME="Москва" TYPENAME="г" ISACTUAL="1" ISACTIVE="1"/>
</OBJECTS>`
	var valid,invalid int
	err:=(Decoder{RegionCode:77,Kind:KindAddress}).StreamRows(strings.NewReader(xmlData),func(row Row)error{
		if row.Err!=nil{invalid++;return nil}
		valid++
		if row.Normalized==nil{t.Fatal("nil normalized row")}
		return nil
	})
	if err!=nil{t.Fatal(err)}
	if valid!=2||invalid!=1{t.Fatalf("valid=%d invalid=%d",valid,invalid)}
}

func TestHierarchyUsesItemParentObjectID(t *testing.T){
	xmlData:=`<ITEMS><ITEM OBJECTID="20" PARENTOBJID="10"/></ITEMS>`
	err:=(Decoder{RegionCode:77,Kind:KindHierarchy}).Stream(strings.NewReader(xmlData),func(_ int64,row Normalized)error{
		if row.ObjectID!=20||row.ParentObjectID!=10{t.Fatalf("row=%+v",row)}
		return nil
	})
	if err!=nil{t.Fatal(err)}
}

func TestHouseRequiresSomeNumber(t *testing.T){
	xmlData:=`<HOUSES><HOUSE OBJECTID="30" ISACTUAL="1" ISACTIVE="1"/></HOUSES>`
	var invalid int
	err:=(Decoder{RegionCode:77,Kind:KindHouse}).StreamRows(strings.NewReader(xmlData),func(row Row)error{if row.Err!=nil{invalid++};return nil})
	if err!=nil{t.Fatal(err)}
	if invalid!=1{t.Fatalf("invalid=%d",invalid)}
}
