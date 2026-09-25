package mail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	HardBounceThreshold = 2
	HardBounceWindow = 30 * 24 * time.Hour
	HardBounceSuppressionTTL = 30 * 24 * time.Hour
)

type DeliveryFailureClass string

const (
	FailureUnknown DeliveryFailureClass = "UNKNOWN"
	FailureTransient DeliveryFailureClass = "TRANSIENT"
	FailureHard DeliveryFailureClass = "HARD"
)

func ClassifyDeliveryFailure(code string) DeliveryFailureClass {
	code = strings.TrimSpace(code)
	if code == "" { return FailureUnknown }
	if len(code) >= 1 {
		switch code[0] {
		case '5':
			if validSMTPStatusPrefix(code) { return FailureHard }
		case '4':
			if validSMTPStatusPrefix(code) { return FailureTransient }
		}
	}
	return FailureUnknown
}

func validSMTPStatusPrefix(code string) bool {
	// Accept RFC 5321 status (550) and RFC 3463 enhanced status (5.1.1 / 4.2.0).
	if len(code) >= 3 && code[0] >= '4' && code[0] <= '5' && code[1] >= '0' && code[1] <= '9' && code[2] >= '0' && code[2] <= '9' { return true }
	if len(code) >= 5 && (code[0] == '4' || code[0] == '5') && code[1] == '.' && code[2] >= '0' && code[2] <= '9' && code[3] == '.' && code[4] >= '0' && code[4] <= '9' { return true }
	return false
}

func deliveryAddressHash(address string) (string,error) {
	normalized,err:=NormalizeExternalAddress(address);if err!=nil{return "",err}
	sum:=sha256.Sum256([]byte(strings.ToLower(normalized)))
	return hex.EncodeToString(sum[:]),nil
}

func isDeliverySuppressedTx(ctx context.Context,tx pgx.Tx,senderMailboxID int64,address string,now time.Time)(bool,error){
	hash,err:=deliveryAddressHash(address);if err!=nil{return false,err}
	var suppressed bool
	err=tx.QueryRow(ctx,`SELECT EXISTS(
 SELECT 1 FROM mail_delivery_suppressions
 WHERE sender_mailbox_id=$1 AND address_sha256=$2 AND cleared_at IS NULL
   AND suppressed_until IS NOT NULL AND suppressed_until>$3
)`,senderMailboxID,hash,now).Scan(&suppressed)
	return suppressed,err
}

func recordHardBounceTx(ctx context.Context,tx pgx.Tx,senderMailboxID int64,address string,now time.Time)(bool,error){
	hash,err:=deliveryAddressHash(address);if err!=nil{return false,err}
	var count int;var first,last *time.Time
	err=tx.QueryRow(ctx,`SELECT hard_bounce_count,first_bounced_at,last_bounced_at
 FROM mail_delivery_suppressions
 WHERE sender_mailbox_id=$1 AND address_sha256=$2 FOR UPDATE`,senderMailboxID,hash).Scan(&count,&first,&last)
	if err==pgx.ErrNoRows{
		_,err=tx.Exec(ctx,`INSERT INTO mail_delivery_suppressions(sender_mailbox_id,address_sha256,reason,hard_bounce_count,first_bounced_at,last_bounced_at,updated_at)
 VALUES($1,$2,'HARD_BOUNCE',1,$3,$3,$3)`,senderMailboxID,hash,now)
		return false,err
	}
	if err!=nil{return false,err}
	if last==nil||now.Sub(*last)>HardBounceWindow{count=0;first=nil}
	count++
	if first==nil{t:=now;first=&t}
	var until *time.Time
	if count>=HardBounceThreshold{t:=now.Add(HardBounceSuppressionTTL);until=&t}
	_,err=tx.Exec(ctx,`UPDATE mail_delivery_suppressions SET reason='HARD_BOUNCE',hard_bounce_count=$3,first_bounced_at=$4,last_bounced_at=$5,
 suppressed_until=$6,cleared_at=NULL,updated_at=$5 WHERE sender_mailbox_id=$1 AND address_sha256=$2`,senderMailboxID,hash,count,*first,now,until)
	return until!=nil,err
}
