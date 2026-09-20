package gar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidRecord = errors.New("invalid GAR record")
	guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Kind string
const (
	KindAddress Kind = "ADDR_OBJ"
	KindHouse Kind = "HOUSE"
	KindHierarchy Kind = "HIERARCHY"
)

type Record struct {
	Kind Kind `json:"kind"`
	RegionCode int `json:"region_code"`
	ObjectID int64 `json:"object_id"`
	ObjectGUID string `json:"object_guid,omitempty"`
	ParentObjectID int64 `json:"parent_object_id,omitempty"`
	Level int `json:"level,omitempty"`
	Name string `json:"name,omitempty"`
	TypeName string `json:"type_name,omitempty"`
	HouseNum string `json:"house_num,omitempty"`
	AddNum1 string `json:"add_num1,omitempty"`
	AddNum2 string `json:"add_num2,omitempty"`
	IsActual *bool `json:"is_actual,omitempty"`
	IsActive *bool `json:"is_active,omitempty"`
}

type Normalized struct {
	Record
	NormalizedName string
	PayloadHash string
	RawPayload []byte
}

func Normalize(record Record)(Normalized,error){
	if record.Kind!=KindAddress&&record.Kind!=KindHouse&&record.Kind!=KindHierarchy{return Normalized{},ErrInvalidRecord}
	if record.RegionCode<1||record.RegionCode>99||record.ObjectID<=0{return Normalized{},ErrInvalidRecord}
	if record.ParentObjectID<0||record.ParentObjectID==record.ObjectID{return Normalized{},ErrInvalidRecord}
	if record.ObjectGUID!=""&&!guidPattern.MatchString(record.ObjectGUID){return Normalized{},ErrInvalidRecord}
	if record.Level<0||record.Level>99{return Normalized{},ErrInvalidRecord}
	record.ObjectGUID=strings.ToLower(strings.TrimSpace(record.ObjectGUID))
	record.Name=compact(record.Name);record.TypeName=compact(record.TypeName)
	record.HouseNum=compact(record.HouseNum);record.AddNum1=compact(record.AddNum1);record.AddNum2=compact(record.AddNum2)
	if utf8.RuneCountInString(record.Name)>500||utf8.RuneCountInString(record.TypeName)>100||utf8.RuneCountInString(record.HouseNum)>100{return Normalized{},ErrInvalidRecord}
	if record.Kind==KindAddress&&(record.Name==""||record.Level==0){return Normalized{},ErrInvalidRecord}
	if record.Kind==KindHierarchy&&record.ParentObjectID==0{return Normalized{},ErrInvalidRecord}
	if record.Kind==KindHouse&&record.HouseNum==""&&record.AddNum1==""&&record.AddNum2==""{return Normalized{},ErrInvalidRecord}
	raw,err:=json.Marshal(record);if err!=nil{return Normalized{},err}
	sum:=sha256.Sum256(raw)
	return Normalized{Record:record,NormalizedName:normalizeText(displayName(record)),PayloadHash:hex.EncodeToString(sum[:]),RawPayload:raw},nil
}

func displayName(record Record)string{
	switch record.Kind{
	case KindAddress:
		if record.TypeName!=""{return compact(record.TypeName+" "+record.Name)}
		return record.Name
	case KindHouse:
		parts:=[]string{}
		if record.HouseNum!=""{parts=append(parts,"д "+record.HouseNum)}
		if record.AddNum1!=""{parts=append(parts,record.AddNum1)}
		if record.AddNum2!=""{parts=append(parts,record.AddNum2)}
		return strings.Join(parts," ")
	default:return strconv.FormatInt(record.ObjectID,10)
	}
}

func normalizeText(value string)string{
	value=strings.ToLower(compact(value));var b strings.Builder;space:=false
	for _,r:=range value{
		if unicode.IsLetter(r)||unicode.IsDigit(r){if space&&b.Len()>0{b.WriteByte(' ')};space=false;b.WriteRune(r)}else{space=true}
	}
	return strings.TrimSpace(b.String())
}
func compact(value string)string{return strings.Join(strings.Fields(strings.TrimSpace(value))," ")}
