package mail

import "testing"

func TestRetryableDeadCode(t *testing.T){
	allowed:=[]string{"SUBMITTED_STALE","MTA_UNAVAILABLE","MTA_RESPONSE","MTA_ERROR","MTA_HTTP_429","MTA_HTTP_500","MTA_HTTP_503"}
	for _,code:=range allowed{if !RetryableDeadCode(code){t.Fatalf("expected retryable code %q",code)}}
	blocked:=[]string{"","RECIPIENT_SUPPRESSED","MTA_CONFIG","ENVELOPE_INVALID","MESSAGE_TOO_LARGE","MTA_HTTP_400","MTA_HTTP_401","MTA_HTTP_404","550","5.1.1"}
	for _,code:=range blocked{if RetryableDeadCode(code){t.Fatalf("unexpected retryable code %q",code)}}
}
