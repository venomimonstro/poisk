package mail

import (
	"errors"
	"testing"
)

func TestSafeFilenameRejectsTraversalAndControls(t *testing.T){
	for _,name:=range []string{"../secret.txt","..\\secret.txt","line\nbreak.txt","line\rbreak.txt","tab\tname.txt",string([]byte{'a',0,'b'})}{
		if _,err:=safeFilename(name);!errors.Is(err,ErrInvalid){t.Fatalf("name=%q err=%v",name,err)}
	}
}

func TestSafeFilenameAcceptsUserFacingName(t *testing.T){
	name,err:=safeFilename("Договор № 12 (final).pdf");if err!=nil{t.Fatal(err)};if name!="Договор № 12 (final).pdf"{t.Fatalf("name=%q",name)}
}
