package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONEscapesStoredHTML(t *testing.T){
	w:=httptest.NewRecorder()
	writeJSON(w,200,map[string]string{"body":"<script>alert('xss')</script><img src=x onerror=alert(1)>"})
	body:=w.Body.String()
	if strings.Contains(strings.ToLower(body),"<script")||strings.Contains(strings.ToLower(body),"<img"){
		t.Fatalf("raw html leaked into JSON response: %s",body)
	}
	if !strings.Contains(body,"\\u003cscript\\u003e"){
		t.Fatalf("expected html-safe JSON escaping, got %s",body)
	}
}
