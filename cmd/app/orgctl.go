package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/organizations"
	orgindex "github.com/venomimonstro/poisk/internal/organizations/indexer"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func organizationImportRoot()string{
	if value:=strings.TrimSpace(os.Getenv("ORG_IMPORT_ROOT"));value!=""{return value}
	return "/imports"
}

func organizationActor()string{
	if value:=strings.TrimSpace(os.Getenv("ORG_ACTOR"));value!=""{return value}
	return "orgctl"
}

func runOrgCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: orgctl source|import|plan|review|promote|status|rebuild-index")}
	repo:=organizations.NewRepository(pool)
	switch args[0]{
	case "source":
		if len(args)!=4{return errors.New("usage: orgctl source <key> <display-name> <trust-0-100>")}
		trust,err:=strconv.Atoi(args[3]);if err!=nil{return err}
		return repo.EnsureSource(ctx,args[1],args[2],trust)
	case "import":
		if len(args)!=5{return errors.New("usage: orgctl import <source-key> <external-batch-key> <dry-run|apply> <relative-jsonl-path>")}
		mode:=strings.ToUpper(strings.ReplaceAll(args[3],"-","_"))
		if mode=="DRY_RUN"{}else if mode=="APPLY"{}else{return organizations.ErrInvalidBatch}
		path,err:=resolveImportPath(organizationImportRoot(),args[4]);if err!=nil{return err}
		batch,err:=repo.CreateBatch(ctx,args[1],args[2],mode);if err!=nil{return err}
		if batch.Status=="STAGING"{
			file,err:=os.Open(path);if err!=nil{return err}
			stageErr:= (organizations.Stager{Store:repo}).StageJSONL(ctx,batch.ID,file)
			closeErr:=file.Close();if stageErr!=nil{return stageErr};if closeErr!=nil{return closeErr}
			batch,err=repo.Batch(ctx,batch.ID);if err!=nil{return err}
		}
		if batch.Status=="PLANNING"{
			if err:= (organizations.Planner{Store:repo,PageSize:100}).PlanBatch(ctx,batch.ID);err!=nil{return err}
		}
		return printOrgSummary(ctx,repo,batch.ID)
	case "plan":
		if len(args)!=2{return errors.New("usage: orgctl plan <batch-id>")}
		batchID,err:=positiveInt64(args[1]);if err!=nil{return err}
		batch,err:=repo.Batch(ctx,batchID);if err!=nil{return err}
		if batch.Status=="PLANNING"{if err:= (organizations.Planner{Store:repo,PageSize:100}).PlanBatch(ctx,batchID);err!=nil{return err}}
		return printOrgSummary(ctx,repo,batchID)
	case "review":
		if len(args)<3{return errors.New("usage: orgctl review <staging-id> merge <place-id> | create | reject")}
		stagingID,err:=positiveInt64(args[1]);if err!=nil{return err}
		decision:=strings.ToLower(args[2])
		var candidate *int64
		switch decision{
		case "merge":
			if len(args)!=4{return errors.New("usage: orgctl review <staging-id> merge <place-id>")}
			placeID,err:=positiveInt64(args[3]);if err!=nil{return err};candidate=&placeID
			decision="MERGE"
		case "create":
			if len(args)!=3{return errors.New("usage: orgctl review <staging-id> create")};decision="CREATE_NEW"
		case "reject":
			if len(args)!=3{return errors.New("usage: orgctl review <staging-id> reject")};decision="REJECT"
		default:return errors.New("review decision must be merge, create or reject")
		}
		return repo.ResolveReview(ctx,stagingID,decision,candidate,organizationActor())
	case "promote":
		if len(args)!=2{return errors.New("usage: orgctl promote <batch-id>")}
		batchID,err:=positiveInt64(args[1]);if err!=nil{return err}
		if err:=repo.PromoteDryRun(ctx,batchID);err!=nil{return err}
		return printOrgSummary(ctx,repo,batchID)
	case "status":
		if len(args)!=2{return errors.New("usage: orgctl status <batch-id>")}
		batchID,err:=positiveInt64(args[1]);if err!=nil{return err}
		return printOrgSummary(ctx,repo,batchID)
	case "rebuild-index":
		if len(args)!=1{return errors.New("usage: orgctl rebuild-index")}
		return rebuildOrganizationIndex(ctx,pool)
	default:return fmt.Errorf("unknown orgctl command %q",args[0])
	}
}

func rebuildOrganizationIndex(ctx context.Context,pool *pgxpool.Pool)error{
	cfg,err:=config.Load();if err!=nil{return err}
	index,err:=indexmanticore.New(indexmanticore.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return err}
	if err:=index.ResetOrganizationsSchema(ctx);err!=nil{return err}
	source:=orgindex.NewSource(pool)
	var after int64
	var applied int
	for{
		docs,err:=source.RebuildBatch(ctx,after,500);if err!=nil{return err}
		if len(docs)==0{break}
		for _,doc:=range docs{
			if _,err:=index.ApplyOrganization(ctx,doc);err!=nil{return fmt.Errorf("rebuild organization %d: %w",doc.ID,err)}
			after=doc.ID;applied++
		}
		if ctx.Err()!=nil{return ctx.Err()}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status":"rebuilt","organizations":applied})
}

func printOrgSummary(ctx context.Context,repo *organizations.Repository,batchID int64)error{
	summary,err:=repo.Summary(ctx,batchID);if err!=nil{return err}
	encoder:=json.NewEncoder(os.Stdout);encoder.SetIndent("","  ");return encoder.Encode(summary)
}

func positiveInt64(raw string)(int64,error){value,err:=strconv.ParseInt(raw,10,64);if err!=nil||value<=0{return 0,organizations.ErrInvalidBatch};return value,nil}

func resolveImportPath(root,relative string)(string,error){
	if strings.TrimSpace(root)==""||relative==""||filepath.IsAbs(relative)||strings.Contains(relative,"\\")||strings.Contains(relative,".."){return "",organizations.ErrInvalidBatch}
	rootAbs,err:=filepath.Abs(root);if err!=nil{return "",err}
	full,err:=filepath.Abs(filepath.Join(rootAbs,filepath.FromSlash(relative)));if err!=nil{return "",err}
	rel,err:=filepath.Rel(rootAbs,full);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",organizations.ErrInvalidBatch}
	return full,nil
}
