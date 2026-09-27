package release

import "testing"

func TestValidManifestFields(t *testing.T){
	goodHash:="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	goodCommit:="0123456789abcdef0123456789abcdef01234567"
	if !ValidManifestFields("2026.09.20",goodCommit,goodHash,"registry.example/poisk/backend@sha256:abc123","registry.example/poisk/frontend:2026.09.20"){t.Fatal("valid manifest rejected")}
	bad:=[]struct{name,version,build,hash,backend,frontend string}{
		{"newline image","1",goodCommit,goodHash,"image\nEVIL=1","front"},
		{"shell image","1",goodCommit,goodHash,"image;touch/tmp/pwn","front"},
		{"bad hash","1",goodCommit,"xyz","backend","front"},
		{"bad version","1 $(id)",goodCommit,goodHash,"backend","front"},
		{"short commit","1","abc123",goodHash,"backend","front"},
		{"uppercase commit","1","0123456789ABCDEF0123456789ABCDEF01234567",goodHash,"backend","front"},
	}
	for _,tc:=range bad{t.Run(tc.name,func(t *testing.T){if ValidManifestFields(tc.version,tc.build,tc.hash,tc.backend,tc.frontend){t.Fatal("unsafe manifest accepted")}})}
}
