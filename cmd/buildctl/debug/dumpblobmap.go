package debug

import (
	"path/filepath"

	"github.com/urfave/cli"
)

var DumpBlobmapCommand = cli.Command{
	Name:  "dump-blobmap",
	Usage: "dump the blobmap in human-readable format.  This command does not require the daemon to be running.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "root",
			Usage: "path to state directory",
			Value: ".buildstate",
		},
	},
	Action: func(clicontext *cli.Context) error {
		dbFile := filepath.Join(clicontext.String("root"), "blobmap", "blobmap.db")
		return dumpBolt(dbFile)
	},
}
