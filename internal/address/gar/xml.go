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

func (d Decoder) Stream(reader io.Reader,visit func(int64,Normalized)error)error{
	if reader==nil||visit==nil||d.RegionCode<1||d.RegionCode>99{return ErrInvalidRecord}
	if d.Kind!=KindAddress&&d.Kind!=KindHouse&&d.Kind!=KindHierarchy{return ErrInvalidRecord}
	dec:=xml.NewDecoder(reader)
	var row int64
	for{
		token,err:=dec.Token()
		if errors.Is(err,io.EOF){return nil}
		if err!=nil{return fmt.Errorf("decode GAR XML: %w",err)}
		start,ok:=token.(xml.StartElement);if !ok{continue}
		if !d.acceptElement(start.Name.Local){continue}
		row++
		record,err:=d.fromAttributes(start.Attr)
		if err!=nil{return fmt.Errorf("GAR row %d: %w",row,err)}
		normalized,err:=Normalize(record);if err!=nil{return fmt.Errorf("GAR row %d: %w",row,err)}
		if err:=visit(row,normalized);err!=nil{return err}
	}
}

func (d Decoder) acceptElement(name string)bool{
	switch d.Kind{case KindAddress:return name=="OBJECT";case KindHouse:return name=="HOUSE";case KindHierarchy:return name=="ITEM"}
	return false
}

func (d Decoder) fromAttributes(attrs []xml.Attr)(Record,error){
	values:=make(map[string]string,len(attrs));for _,attr:=range attrs{values[strings.ToUpper(attr.Name.Local)]=strings.TrimSpace(attr.Value)}
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
