package mail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	stdmail "net/mail"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	MaxInboundRawBytes = 20 << 20
	MaxInboundParts = 64
	MaxInboundAttachments = 20
	MaxInboundAttachmentBytes int64 = 25 << 20
	MaxInboundAttachmentTotal int64 = 50 << 20
)

var ErrDangerousMailPart = errors.New("dangerous mail part")

type InboundAttachment struct {
	Filename string
	ContentType string
	Data []byte
}

type InboundMessage struct {
	From string
	ReplyTo string
	InternetMessageID string
	Subject string
	BodyText string
	Attachments []InboundAttachment
}

type inboundBudget struct{parts int;attachments int;attachmentBytes int64}

func ParseInboundMIME(raw []byte)(InboundMessage,error){
	if len(raw)==0||len(raw)>MaxInboundRawBytes{return InboundMessage{},ErrInvalid}
	msg,err:=stdmail.ReadMessage(bytes.NewReader(raw));if err!=nil{return InboundMessage{},ErrInvalid}
	from,err:=singleMailboxAddress(msg.Header.Get("From"));if err!=nil{return InboundMessage{},ErrInvalid}
	replyTo:="";if strings.TrimSpace(msg.Header.Get("Reply-To"))!=""{replyTo,err=singleMailboxAddress(msg.Header.Get("Reply-To"));if err!=nil{return InboundMessage{},ErrInvalid}}
	internetID:=strings.TrimSpace(msg.Header.Get("Message-ID"));if len(internetID)>998||strings.ContainsAny(internetID,"\r\n\x00"){return InboundMessage{},ErrInvalid}
	subject:=decodeHeader(msg.Header.Get("Subject"));subject=strings.TrimSpace(subject);if subject==""{subject="(без темы)"};if utf8.RuneCountInString(subject)>maxSubjectRunes{return InboundMessage{},ErrInvalid}
	mediaType,params,err:=mime.ParseMediaType(msg.Header.Get("Content-Type"));if err!=nil&&msg.Header.Get("Content-Type")!=""{return InboundMessage{},ErrInvalid};if mediaType==""{mediaType="text/plain"}
	var textParts []string;var attachments []InboundAttachment;budget:=&inboundBudget{}
	if err:=parseInboundEntity(mediaType,params,msg.Header,msg.Body,budget,&textParts,&attachments,0);err!=nil{return InboundMessage{},err}
	body:=strings.TrimSpace(strings.Join(textParts,"\n\n"));if body==""{body="[Письмо не содержит безопасной текстовой части]"};if utf8.RuneCountInString(body)>maxBodyRunes{body=string([]rune(body)[:maxBodyRunes])}
	return InboundMessage{From:from,ReplyTo:replyTo,InternetMessageID:internetID,Subject:subject,BodyText:body,Attachments:attachments},nil
}

func parseInboundEntity(mediaType string,params map[string]string,header stdmail.Header,body io.Reader,b *inboundBudget,textParts *[]string,attachments *[]InboundAttachment,depth int)error{
	if depth>12{return ErrInvalid};b.parts++;if b.parts>MaxInboundParts{return ErrInvalid}
	mediaType=strings.ToLower(strings.TrimSpace(mediaType));disposition,dispParams,_:=mime.ParseMediaType(header.Get("Content-Disposition"));filename:=decodeHeader(dispParams["filename"]);if filename==""{filename=decodeHeader(params["name"])}
	if strings.HasPrefix(mediaType,"multipart/"){
		boundary:=params["boundary"];if boundary==""{return ErrInvalid};mr:=multipart.NewReader(body,boundary)
		for{part,err:=mr.NextPart();if errors.Is(err,io.EOF){break};if err!=nil{return ErrInvalid};ct,p,e:=mime.ParseMediaType(part.Header.Get("Content-Type"));if e!=nil&&part.Header.Get("Content-Type")!=""{_ = part.Close();return ErrInvalid};if ct==""{ct="text/plain"};h:=stdmail.Header(part.Header);if err=parseInboundEntity(ct,p,h,part,b,textParts,attachments,depth+1);err!=nil{_ = part.Close();return err};_ = part.Close()};return nil
	}
	isAttachment:=strings.EqualFold(disposition,"attachment")||filename!=""
	decoded:=decodeTransfer(body,header.Get("Content-Transfer-Encoding"));if decoded==nil{return ErrInvalid}
	if isAttachment{
		if filename==""{filename="attachment"};safe,err:=safeFilename(filename);if err!=nil{return err};if dangerousAttachment(safe,mediaType){return ErrDangerousMailPart}
		b.attachments++;if b.attachments>MaxInboundAttachments{return ErrInvalid}
		data,err:=io.ReadAll(io.LimitReader(decoded,MaxInboundAttachmentBytes+1));if err!=nil||len(data)==0||int64(len(data))>MaxInboundAttachmentBytes{return ErrInvalid};b.attachmentBytes+=int64(len(data));if b.attachmentBytes>MaxInboundAttachmentTotal{return ErrInvalid}
		*attachments=append(*attachments,InboundAttachment{Filename:safe,ContentType:normalizeContentType(mediaType),Data:data});return nil
	}
	if mediaType=="text/plain"{
		data,err:=io.ReadAll(io.LimitReader(decoded,2<<20));if err!=nil{return ErrInvalid};charset:=strings.ToLower(strings.TrimSpace(params["charset"]));if charset!=""&&charset!="utf-8"&&charset!="us-ascii"{return nil};text:=strings.TrimSpace(string(data));if text!=""{*textParts=append(*textParts,text)};return nil
	}
	return nil
}

func decodeTransfer(r io.Reader,enc string)io.Reader{switch strings.ToLower(strings.TrimSpace(enc)){case "","7bit","8bit","binary":return r;case "base64":return base64.NewDecoder(base64.StdEncoding,r);case "quoted-printable":return quotedprintable.NewReader(r);default:return nil}}

func singleMailboxAddress(raw string)(string,error){list,err:=stdmail.ParseAddressList(raw);if err!=nil||len(list)!=1{return "",ErrInvalid};addr:=strings.TrimSpace(list[0].Address);if len(addr)<3||len(addr)>320||strings.ContainsAny(addr,"\r\n\x00"){return "",ErrInvalid};return addr,nil}
func decodeHeader(raw string)string{if strings.TrimSpace(raw)==""{return ""};v,err:=new(mime.WordDecoder).DecodeHeader(raw);if err!=nil{return raw};return v}
func dangerousAttachment(name,contentType string)bool{ext:=strings.ToLower(filepath.Ext(name));switch ext{case ".exe",".com",".bat",".cmd",".ps1",".vbs",".vbe",".js",".jse",".wsf",".wsh",".scr",".msi",".msp",".jar",".hta",".html",".htm",".svg":return true};ct:=strings.ToLower(strings.TrimSpace(contentType));return ct=="text/html"||ct=="image/svg+xml"||strings.Contains(ct,"javascript")||strings.Contains(ct,"x-msdownload")}
func InboundRecipientAddress(raw string)(string,error){raw=strings.ToLower(strings.TrimSpace(raw));if len(raw)<3||len(raw)>320||strings.ContainsAny(raw,"\r\n\x00"){return "",ErrInvalid};parsed,err:=stdmail.ParseAddress(raw);if err!=nil||strings.ToLower(parsed.Address)!=raw{return "",ErrInvalid};return raw,nil}
func inboundDuplicateKey(eventID string)string{return fmt.Sprintf("inbound:%s",strings.TrimSpace(eventID))}
