package debug

import (
	"path/filepath"

	"github.com/urfave/cli"
)

var DumpCacheCommand = cli.Command{
	Name:  "dump-cache",
	Usage: "dump cache metadata in human-readable format.  This command does not require the daemon to be running.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "root",
			Usage: "path to state directory",
			Value: ".buildstate",
		},
	},
	Action: func(clicontext *cli.Context) error {
		dbFile := filepath.Join(clicontext.String("root"), "cachemanager", "cache.db")
		return dumpBolt(dbFile)
	},
}
