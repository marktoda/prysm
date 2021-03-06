package flags

import (
	"github.com/prysmaticlabs/prysm/shared/cmd"
	log "github.com/sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
)

// GlobalFlags specifies all the global flags for the
// beacon node.
type GlobalFlags struct {
	EnableArchive                     bool
	EnableArchivedValidatorSetChanges bool
	EnableArchivedBlocks              bool
	EnableArchivedAttestations        bool
	MinimumSyncPeers                  int
	MaxPageSize                       int
	DeploymentBlock                   int
	UnsafeSync                        bool
	EnableDiscv5                      bool
}

var globalConfig *GlobalFlags

// Get retrieves the global config.
func Get() *GlobalFlags {
	if globalConfig == nil {
		return &GlobalFlags{}
	}
	return globalConfig
}

// Init sets the global config equal to the config that is passed in.
func Init(c *GlobalFlags) {
	globalConfig = c
}

// ConfigureGlobalFlags initializes the global config.
// based on the provided cli context.
func ConfigureGlobalFlags(ctx *cli.Context) {
	cfg := &GlobalFlags{}
	if ctx.Bool(ArchiveEnableFlag.Name) {
		cfg.EnableArchive = true
	}
	if ctx.Bool(ArchiveValidatorSetChangesFlag.Name) {
		cfg.EnableArchivedValidatorSetChanges = true
	}
	if ctx.Bool(ArchiveBlocksFlag.Name) {
		cfg.EnableArchivedBlocks = true
	}
	if ctx.Bool(ArchiveAttestationsFlag.Name) {
		cfg.EnableArchivedAttestations = true
	}
	if ctx.Bool(UnsafeSync.Name) {
		cfg.UnsafeSync = true
	}
	if ctx.Bool(EnableDiscv5.Name) {
		cfg.EnableDiscv5 = true
	}
	cfg.MaxPageSize = ctx.Int(RPCMaxPageSize.Name)
	cfg.DeploymentBlock = ctx.Int(ContractDeploymentBlock.Name)
	configureMinimumPeers(ctx, cfg)

	Init(cfg)
}

func configureMinimumPeers(ctx *cli.Context, cfg *GlobalFlags) {
	cfg.MinimumSyncPeers = ctx.Int(MinSyncPeers.Name)
	maxPeers := int(ctx.Int64(cmd.P2PMaxPeers.Name))
	if cfg.MinimumSyncPeers > maxPeers {
		log.Warnf("Changing Minimum Sync Peers to %d", maxPeers)
		cfg.MinimumSyncPeers = maxPeers
	}
}
