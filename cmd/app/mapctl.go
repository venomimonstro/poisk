package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	mapsvc "github.com/venomimonstro/poisk/internal/maps"
)

func mapArtifactRoot() string {
	if v := strings.TrimSpace(os.Getenv("MAP_ARTIFACT_ROOT")); v != "" {
		return v
	}
	return "/maps"
}

func newMapService(pool *pgxpool.Pool) *mapsvc.Service {
	return &mapsvc.Service{
		Store: mapsvc.NewRepository(pool),
		ArtifactRoot: mapArtifactRoot(),
		PublicPrefix: "/maps",
	}
}

func runMapCtl(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mapctl manifest|register|activate|rollback|status")
	}
	service := newMapService(pool)
	actor := strings.TrimSpace(os.Getenv("MAP_ACTOR"))
	if actor == "" { actor = "mapctl" }

	switch args[0] {
	case "manifest":
		if len(args) != 6 {
			return errors.New("usage: mapctl manifest <version> <pmtiles-path> <style-path> <source-key> <attribution>")
		}
		manifest, err := mapsvc.BuildManifest(service.ArtifactRoot,args[1],args[2],args[3],args[4],args[5])
		if err != nil { return fmt.Errorf("build map manifest: %w",err) }
		encoder:=json.NewEncoder(os.Stdout);encoder.SetIndent("","  ")
		return encoder.Encode(manifest)
	case "register":
		if len(args) != 2 { return errors.New("usage: mapctl register <manifest>") }
		manifest, err := mapsvc.LoadManifest(service.ArtifactRoot, args[1])
		if err != nil { return fmt.Errorf("load map manifest: %w", err) }
		if err := service.Register(ctx, manifest, actor); err != nil { return err }
		fmt.Printf("registered map version %s\n", manifest.Version)
		return nil
	case "activate":
		if len(args) != 2 { return errors.New("usage: mapctl activate <version>") }
		if err := service.Activate(ctx, args[1], actor); err != nil { return err }
		fmt.Printf("activated map version %s\n", args[1])
		return nil
	case "rollback":
		if len(args) != 1 { return errors.New("usage: mapctl rollback") }
		version, err := service.Rollback(ctx, actor)
		if err != nil { return err }
		fmt.Printf("rolled back to map version %s\n", version)
		return nil
	case "status":
		if len(args) != 1 { return errors.New("usage: mapctl status") }
		cfg, err := service.ActiveConfig(ctx)
		if err != nil { return err }
		fmt.Printf("active=%s pmtiles=%s style=%s\n", cfg.Version, cfg.PMTilesURL, cfg.StyleURL)
		return nil
	default:
		return fmt.Errorf("unknown mapctl command %q", args[0])
	}
}
