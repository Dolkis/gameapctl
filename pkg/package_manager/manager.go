package packagemanager

import (
	"context"

	contextInternal "github.com/gameap/gameapctl/internal/context"
)

type Package struct {
	Name             string
	Status           string
	Architecture     string
	Version          string
	ShortDescription string
	InstalledSizeKB  int
}

type PackageManager interface {
	Search(ctx context.Context, name string) ([]*Package, error)
	Install(ctx context.Context, packs ...string) error
	CheckForUpdates(ctx context.Context) error
	Remove(ctx context.Context, packs ...string) error
}

func Load(ctx context.Context) (PackageManager, error) {
	osInfo := contextInternal.OSInfoFromContext(ctx)

	switch osInfo.Distribution {
	case "debian", "ubuntu":
		return NewExtendedAPT(&APT{}), nil
	}
	return nil, nil
}
