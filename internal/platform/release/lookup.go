package release

import "context"

func (r Repository) ByVersion(ctx context.Context, version string) (Manifest, error) {
	manifest, err := r.byVersion(ctx, version)
	if err != nil { return Manifest{}, err }
	if !ValidManifestFields(manifest.Version, manifest.BuildSHA, manifest.ConfigHash, manifest.BackendImage, manifest.FrontendImage) {
		return Manifest{}, ErrPreflight
	}
	return manifest, nil
}
