package mail

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLFromTextEscapesActiveMarkup(t *testing.T){
	raw:=`<script>alert("x")</script><img src=x onerror=alert(1)> & hello
next`
	got:=sanitizeHTMLFromText(raw)
	for _,forbidden:=range []string{"<script","<img","onerror=","</script>"}{if strings.Contains(strings.ToLower(got),forbidden){t.Fatalf("unsafe html survived: %q",got)}}
	if !strings.Contains(got,"&lt;script&gt;")||!strings.Contains(got,"&amp; hello")||!strings.Contains(got,"<br>"){t.Fatalf("sanitized html=%q",got)}
}

func TestSafeFilenameRejectsTraversalAndControlInput(t *testing.T){
	bad:=[]string{"../a.txt","..\\a.txt",".","..","a/b.txt","a\\b.txt","bad\x00.txt"}
	for _,name:=range bad{if _,err:=safeFilename(name);err==nil{t.Fatalf("filename %q accepted",name)}}
	if got,err:=safeFilename("Фото 2026.png");err!=nil||got!="Фото 2026.png"{t.Fatalf("valid filename got=%q err=%v",got,err)}
}

func TestNormalizeContentTypeRejectsHeaderInjection(t *testing.T){
	for _,raw:=range []string{"text/plain\r\nX-Evil: 1","text/plain\x00evil",""}{if got:=normalizeContentType(raw);got!="application/octet-stream"{t.Fatalf("content type %q -> %q",raw,got)}}
	if got:=normalizeContentType("image/png");got!="image/png"{t.Fatalf("valid content type=%q",got)}
}
