package debug

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var DumpBoltCommand = cli.Command{
	Name:  "dump-bolt",
	Usage: "dump an arbitrary bolt db file in human-readable format.  This command does not require the daemon to be running.",
	// unlike dumpLLB(), dumpBolt() does not support reading from stdin
	ArgsUsage: "<dbfile>",
	Action: func(clicontext *cli.Context) error {
		dbFile := clicontext.Args().First()
		return dumpBolt(dbFile)
	},
}

func dumpBolt(dbFile string) error {
	if dbFile == "" {
		return errors.New("dbfile not specified")
	}
	if dbFile == "-" {
		// user could still specify "/dev/stdin" but unlikely to work
		return errors.New("stdin unsupported")
	}
	db, err := bolt.Open(dbFile, 0400, &bolt.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		// TODO: JSON format?
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			fmt.Printf("bucket %q:\n", string(name))
			return b.ForEach(func(k, v []byte) error {
				fmt.Printf("  %q: %q\n", string(k), string(v))
				return nil
			})
		})
	})
}
