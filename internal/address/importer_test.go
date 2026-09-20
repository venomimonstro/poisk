package address

import (
	"context"
	"strings"
	"testing"

	"github.com/venomimonstro/poisk/internal/address/gar"
)

type importStoreFake struct{
	batch Batch
	staged []int64
	rejected []int64
	checkpoints []int64
}
func (s *importStoreFake) Batch(context.Context,int64)(Batch,error){return s.batch,nil}
func (s *importStoreFake) Stage(_ context.Context,_ int64,_ string,row int64,_ gar.Normalized)(int64,error){s.staged=append(s.staged,row);return row,nil}
func (s *importStoreFake) Reject(_ context.Context,_ int64,_ string,row int64,_ gar.Kind,_ int,_ map[string]string,_ string,_ string)(int64,error){s.rejected=append(s.rejected,row);return row,nil}
func (s *importStoreFake) AdvanceCheckpoint(_ context.Context,_ int64,_ string,row int64)error{s.checkpoints=append(s.checkpoints,row);return nil}

func TestImporterResumesSameFileAndRejectsBadRow(t *testing.T){
	store:=&importStoreFake{batch:Batch{ID:1,Status:"STAGING",CheckpointFile:"AS_ADDR_OBJ.xml",CheckpointRow:1}}
	input:=`<OBJECTS>
<OBJECT OBJECTID="1" LEVEL="8" NAME="Old" TYPENAME="ул"/>
<OBJECT OBJECTID="bad" LEVEL="8" NAME="Bad" TYPENAME="ул"/>
<OBJECT OBJECTID="3" LEVEL="8" NAME="New" TYPENAME="ул"/>
</OBJECTS>`
	importer:=Importer{Store:store,CheckpointEvery:2}
	err:=importer.ImportFile(context.Background(),1,FileSpec{Name:"AS_ADDR_OBJ.xml",RegionCode:77,Kind:gar.KindAddress},strings.NewReader(input))
	if err!=nil{t.Fatal(err)}
	if len(store.staged)!=1||store.staged[0]!=3{t.Fatalf("staged=%v",store.staged)}
	if len(store.rejected)!=1||store.rejected[0]!=2{t.Fatalf("rejected=%v",store.rejected)}
	if len(store.checkpoints)==0||store.checkpoints[len(store.checkpoints)-1]!=3{t.Fatalf("checkpoints=%v",store.checkpoints)}
}

func TestImporterIgnoresCheckpointFromAnotherFile(t *testing.T){
	store:=&importStoreFake{batch:Batch{ID:1,Status:"STAGING",CheckpointFile:"old.xml",CheckpointRow:999}}
	input:=`<HOUSES><HOUSE OBJECTID="4" HOUSENUM="10" ISACTUAL="1" ISACTIVE="1"/></HOUSES>`
	err:=(Importer{Store:store}).ImportFile(context.Background(),1,FileSpec{Name:"AS_HOUSES.xml",RegionCode:77,Kind:gar.KindHouse},strings.NewReader(input))
	if err!=nil{t.Fatal(err)}
	if len(store.staged)!=1||store.staged[0]!=1{t.Fatalf("staged=%v",store.staged)}
}
