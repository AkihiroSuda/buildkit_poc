package debug

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/urfave/cli"
)

var WorkersCommand = cli.Command{
	Name:   "workers",
	Usage:  "list workers",
	Action: listWorkers,
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "filter, f",
			Usage: "containerd-style filter string slice",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Verbose output",
		},
	},
}

func resolveClient(c *cli.Context) (*client.Client, error) {
	return client.New(c.GlobalString("addr"), client.WithBlock())
}

func listWorkers(clicontext *cli.Context) error {
	c, err := resolveClient(clicontext)
	if err != nil {
		return err
	}

	workers, err := c.ListWorkers(appcontext.Context(), client.WithWorkerFilter(clicontext.StringSlice("filter")))
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 1, 8, 1, '\t', 0)

	if clicontext.Bool("verbose") {
		printWorkersVerbose(tw, workers)
	} else {
		printWorkersTable(tw, workers)
	}
	return nil
}

func printWorkersVerbose(tw *tabwriter.Writer, winfo []*client.WorkerInfo) {
	for _, wi := range winfo {
		fmt.Fprintf(tw, "ID:\t%s\n", wi.ID)
		fmt.Fprintf(tw, "Labels:\n")
		for _, kv := range sortMap(wi.Labels) {
			fmt.Fprintf(tw, "\t%s:\t%s\n", kv[0], kv[1])
		}
		fmt.Fprintf(tw, "\n")
	}

	tw.Flush()
}

func printWorkersTable(tw *tabwriter.Writer, winfo []*client.WorkerInfo) {
	fmt.Fprintln(tw, "ID")

	for _, wi := range winfo {
		id := wi.ID
		fmt.Fprintf(tw, "%s\n", id)
	}

	tw.Flush()
}

func sortMap(m map[string]string) [][2]string {
	var s [][2]string
	for k, v := range m {
		s = append(s, [2]string{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i][0] < s[j][0]
	})
	return s
}
