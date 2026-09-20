package gar

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Decoder struct {
	RegionCode int
	Kind Kind
}

type Row struct {
	Number int64
	Raw map[string]string
	Normalized *Normalized
	Err error
}

func (d Decoder) StreamRows(reader io.Reader,visit func(Row)error)error{
	if reader==nil||visit==nil||d.RegionCode<1||d.RegionCode>99{return ErrInvalidRecord}
	if d.Kind!=KindAddress&&d.Kind!=KindHouse&&d.Kind!=KindHierarchy{return ErrInvalidRecord}
	dec:=xml.NewDecoder(reader)
	var rowNumber int64
	for{
		token,err:=dec.Token()
		if errors.Is(err,io.EOF){return nil}
		if err!=nil{return fmt.Errorf("decode GAR XML: %w",err)}
		start,ok:=token.(xml.StartElement);if !ok||!d.acceptElement(start.Name.Local){continue}
		rowNumber++
		raw:=attrsMap(start.Attr)
		record,rowErr:=d.fromValues(raw)
		if rowErr!=nil{if err:=visit(Row{Number:rowNumber,Raw:raw,Err:rowErr});err!=nil{return err};continue}
		normalized,rowErr:=Normalize(record)
		if rowErr!=nil{if err:=visit(Row{Number:rowNumber,Raw:raw,Err:rowErr});err!=nil{return err};continue}
		if err:=visit(Row{Number:rowNumber,Raw:raw,Normalized:&normalized});err!=nil{return err}
	}
}

func (d Decoder) Stream(reader io.Reader,visit func(int64,Normalized)error)error{
	return d.StreamRows(reader,func(row Row)error{
		if row.Err!=nil{return fmt.Errorf("GAR row %d: %w",row.Number,row.Err)}
		return visit(row.Number,*row.Normalized)
	})
}

func (d Decoder) acceptElement(name string)bool{
	switch d.Kind{case KindAddress:return name=="OBJECT";case KindHouse:return name=="HOUSE";case KindHierarchy:return name=="ITEM"}
	return false
}

func attrsMap(attrs []xml.Attr)map[string]string{
	values:=make(map[string]string,len(attrs));for _,attr:=range attrs{values[strings.ToUpper(attr.Name.Local)]=strings.TrimSpace(attr.Value)};return values
}

func (d Decoder) fromValues(values map[string]string)(Record,error){
	objectID,err:=positiveInt64(values["OBJECTID"]);if err!=nil{return Record{},err}
	record:=Record{Kind:d.Kind,RegionCode:d.RegionCode,ObjectID:objectID,ObjectGUID:values["OBJECTGUID"]}
	if raw:=values["PARENTOBJID"];raw!=""{record.ParentObjectID,err=positiveInt64(raw);if err!=nil{return Record{},err}}
	if raw:=values["LEVEL"];raw!=""{value,e:=strconv.Atoi(raw);if e!=nil{return Record{},ErrInvalidRecord};record.Level=value}
	record.Name=values["NAME"];record.TypeName=values["TYPENAME"]
	record.HouseNum=values["HOUSENUM"];record.AddNum1=values["ADDNUM1"];record.AddNum2=values["ADDNUM2"]
	if raw:=values["ISACTUAL"];raw!=""{value,e:=parseFlag(raw);if e!=nil{return Record{},e};record.IsActual=&value}
	if raw:=values["ISACTIVE"];raw!=""{value,e:=parseFlag(raw);if e!=nil{return Record{},e};record.IsActive=&value}
	return record,nil
}

func positiveInt64(raw string)(int64,error){value,err:=strconv.ParseInt(strings.TrimSpace(raw),10,64);if err!=nil||value<=0{return 0,ErrInvalidRecord};return value,nil}
func parseFlag(raw string)(bool,error){switch strings.TrimSpace(raw){case "1","true","TRUE":return true,nil;case "0","false","FALSE":return false,nil;default:return false,ErrInvalidRecord}}
