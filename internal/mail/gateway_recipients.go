package mail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type internalSendRecipient struct{MailboxID int64;Address,Kind,Status string}
type externalSendRecipient struct{ID int64;Address,Kind string}

func replaceMessageRecipients(ctx context.Context,tx pgx.Tx,messageID int64,items []Recipient) error {
	if _,err:=tx.Exec(ctx,`DELETE FROM mail_recipients WHERE message_id=$1`,messageID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`DELETE FROM mail_external_recipients WHERE message_id=$1`,messageID);err!=nil{return err}
	for i,item:=range items{
		address:=strings.TrimSpace(item.Address);kind:=strings.ToUpper(strings.TrimSpace(item.Type));if kind==""{kind="TO"}
		var mailboxID int64;var canonical string
		err:=tx.QueryRow(ctx,`SELECT mailbox_id,address FROM (
 SELECT mb.mailbox_id,mb.address,0 AS priority FROM mailboxes mb WHERE lower(mb.address)=lower($1) AND mb.status='ACTIVE'
 UNION ALL
 SELECT mb.mailbox_id,a.address,1 AS priority FROM mail_external_aliases a JOIN mailboxes mb ON mb.mailbox_id=a.mailbox_id WHERE lower(a.address)=lower($1) AND a.status='ACTIVE' AND mb.status='ACTIVE'
) x ORDER BY priority LIMIT 1`,address).Scan(&mailboxID,&canonical)
		if err==nil{if _,err=tx.Exec(ctx,`INSERT INTO mail_recipients(message_id,recipient_mailbox_id,recipient_type,ordinal,delivery_address) VALUES($1,$2,$3,$4,$5)`,messageID,mailboxID,kind,i+1,canonical);err!=nil{return err};continue}
		if !errors.Is(err,pgx.ErrNoRows){return err}
		lower:=strings.ToLower(address);if strings.HasSuffix(lower,"@internal.poisk"){return ErrNotFound}
		external,err:=NormalizeExternalAddress(address);if err!=nil{return err};at:=strings.LastIndexByte(external,'@');domain:=external[at+1:]
		var localDomain bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_external_aliases WHERE lower(domain)=lower($1))`,domain).Scan(&localDomain);err!=nil{return err};if localDomain{return ErrNotFound}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_external_recipients(message_id,address,recipient_type,ordinal) VALUES($1,$2,$3,$4)`,messageID,external,kind,i+1);err!=nil{return err}
	}
	return nil
}

func lockSendRecipients(ctx context.Context,tx pgx.Tx,messageID int64)([]internalSendRecipient,[]externalSendRecipient,error){
	rows,err:=tx.Query(ctx,`SELECT mb.mailbox_id,r.delivery_address,r.recipient_type,mb.status FROM mail_recipients r JOIN mailboxes mb ON mb.mailbox_id=r.recipient_mailbox_id WHERE r.message_id=$1 ORDER BY r.recipient_type,r.ordinal FOR SHARE OF r,mb`,messageID);if err!=nil{return nil,nil,err}
	internal:=[]internalSendRecipient{};for rows.Next(){var x internalSendRecipient;if err=rows.Scan(&x.MailboxID,&x.Address,&x.Kind,&x.Status);err!=nil{rows.Close();return nil,nil,err};if x.Status!="ACTIVE"{rows.Close();return nil,nil,ErrConflict};internal=append(internal,x)};if err=rows.Err();err!=nil{rows.Close();return nil,nil,err};rows.Close()
	extRows,err:=tx.Query(ctx,`SELECT external_recipient_id,address,recipient_type FROM mail_external_recipients WHERE message_id=$1 ORDER BY recipient_type,ordinal FOR SHARE`,messageID);if err!=nil{return nil,nil,err}
	external:=[]externalSendRecipient{};for extRows.Next(){var x externalSendRecipient;if err=extRows.Scan(&x.ID,&x.Address,&x.Kind);err!=nil{extRows.Close();return nil,nil,err};external=append(external,x)};if err=extRows.Err();err!=nil{extRows.Close();return nil,nil,err};extRows.Close()
	total:=len(internal)+len(external);if total==0||total>maxRecipients{return nil,nil,ErrInvalid};return internal,external,nil
}

func applySendRecipients(ctx context.Context,tx pgx.Tx,messageID,senderMailboxID int64,internal []internalSendRecipient,external []externalSendRecipient,out *Message) error{
	if len(external)>0{
		var allowed bool
		if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_external_aliases a JOIN mailboxes m ON m.mailbox_id=a.mailbox_id WHERE a.mailbox_id=$1 AND a.is_primary AND a.status='ACTIVE' AND m.status='ACTIVE')`,senderMailboxID).Scan(&allowed);err!=nil{return err};if !allowed{return ErrForbidden}
	}
	now:=time.Now().UTC()
	for _,recipient:=range internal{inboxID,err:=folderID(ctx,tx,recipient.MailboxID,"INBOX");if err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'DELIVERY',$3,FALSE)`,recipient.MailboxID,messageID,inboxID);err!=nil{return err};out.Recipients=append(out.Recipients,Recipient{Address:recipient.Address,Type:recipient.Kind})}
	for _,recipient:=range external{
		suppressed,err:=isDeliverySuppressedTx(ctx,tx,senderMailboxID,recipient.Address,now);if err!=nil{return err};if suppressed{return ErrConflict}
		key:=outboundIdempotency(messageID,recipient.ID,recipient.Address);var deliveryID int64;err=tx.QueryRow(ctx,`INSERT INTO mail_outbound_deliveries(message_id,external_recipient_id,sender_mailbox_id,idempotency_key) VALUES($1,$2,$3,$4) ON CONFLICT(external_recipient_id) DO UPDATE SET external_recipient_id=EXCLUDED.external_recipient_id RETURNING delivery_id`,messageID,recipient.ID,senderMailboxID,key).Scan(&deliveryID);if err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt) SELECT $1,'QUEUE',0 WHERE NOT EXISTS(SELECT 1 FROM mail_outbound_events WHERE delivery_id=$1 AND action='QUEUE')`,deliveryID);err!=nil{return err};out.Recipients=append(out.Recipients,Recipient{Address:recipient.Address,Type:recipient.Kind})
	}
	return nil
}
