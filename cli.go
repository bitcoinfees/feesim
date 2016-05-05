package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/bitcoinfees/feesim/api"
)

func stop(args []string, c *api.Client) {
	const usage = `
feesim stop

Stop the program.
`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}
	if err := c.Stop(); err != nil {
		log.Fatal(err)
	}
}

func status(args []string, c *api.Client) {
	const usage = `
feesim status

Show application status:

	result     : Whether or not a fee estimate is available. Depends on txsource,
	             blocksource and mempool.
	txsource   : Whether or not a transaction source estimate is available.
	blocksource: Whether or not a block source estimate is available.
	mempool    : Whether or not mempool data is available.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	result, err := c.Status()
	if err != nil {
		log.Fatal(err)
	}

	for _, k := range []string{"result", "txsource", "blocksource", "mempool"} {
		fmt.Printf("%-12s: %s\n", k, result[k])
	}
}

func estimateFee(args []string, c *api.Client) {
	const usage = `
feesim estimatefee [N]

Returns the required fee rate (in BTC/kB) for confirmation in N blocks.
If N is omitted, give the result for all available N.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	var n int
	nStr := f.Arg(0)
	if nStr != "" {
		var err error
		n, err = strconv.Atoi(nStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	result, err := c.EstimateFee(n)
	if err != nil {
		log.Fatal(err)
	}

	if n == 0 {
		result := result.([]interface{})
		for i, feerate := range result {
			fmt.Printf("%2d: %10.8f\n", i+1, feerate.(float64))
		}
	} else {
		fmt.Printf("%10.8f\n", result.(float64))
	}
}

func scores(args []string, c *api.Client) {
	const usage = `
feesim scores

Show prediction scores - the proportion of transactions which were confirmed
within their predicted time, as a function of the predicted confirmation time.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	result, err := c.Scores()
	if err != nil {
		log.Fatal(err)
	}

	for i, numAttained := range result["attained"] {
		numTotal := result["exceeded"][i] + numAttained
		fmt.Printf("%2d: %5.3f (%d/%d)\n",
			i+1, numAttained/numTotal, int(numAttained), int(numTotal))
	}
}

func txRate(args []string, c *api.Client) {
	const usage = `
feesim txrate [numpoints]

Show the reverse cumulative tx byterate (bytes/s) as a function of fee rate (sats/kB).

numpoints is an optional integer argument that specifies the number of points on the
function to return.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	var n int
	nStr := f.Arg(0)
	if nStr != "" {
		var err error
		n, err = strconv.Atoi(nStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	result, err := c.TxRate(n)
	if err != nil {
		log.Fatal(err)
	}

	for i, feerate := range result["x"] {
		fmt.Printf("%8d: %8.2f\n", int(feerate), result["y"][i])
	}
}

func capRate(args []string, c *api.Client) {
	const usage = `
feesim caprate [numpoints]

Show the cumulative capacity byterate (bytes/s) as a function of fee rate (sats/kB).

numpoints is an optional integer argument that specifies the number of points on the
function to return.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	var n int
	nStr := f.Arg(0)
	if nStr != "" {
		var err error
		n, err = strconv.Atoi(nStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	result, err := c.CapRate(n)
	if err != nil {
		log.Fatal(err)
	}

	for i, feerate := range result["x"] {
		fmt.Printf("%8d: %8.2f\n", int(feerate), result["y"][i])
	}
}

func mempoolSize(args []string, c *api.Client) {
	const usage = `
feesim mempoolsize [numpoints]

Show the cumulative mempool size (bytes) as a function of fee rate (sats/kB).

numpoints is an optional integer argument that specifies the number of points on the
function to return.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	var n int
	nStr := f.Arg(0)
	if nStr != "" {
		var err error
		n, err = strconv.Atoi(nStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	result, err := c.MempoolSize(n)
	if err != nil {
		log.Fatal(err)
	}

	for i, feerate := range result["x"] {
		fmt.Printf("%8d: %9d\n", int(feerate), int(result["y"][i]))
	}
}

func pause(args []string, c *api.Client) {
	const usage = `
feesim pause

Pause the simulation.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	if err := c.Pause(); err != nil {
		log.Fatal(err)
	}
}

func unpause(args []string, c *api.Client) {
	const usage = `
feesim unpause

Unpause the simulation.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	if err := c.Unpause(); err != nil {
		log.Fatal(err)
	}
}

func setDebug(args []string, c *api.Client) {
	const usage = `
feesim setdebug BOOL

Turn on debug-level logging with "true"; turn off with "false".

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}
	on, err := strconv.ParseBool(f.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	if err := c.SetDebug(on); err != nil {
		log.Fatal(err)
	}
}

func appConfig(args []string, c *api.Client) {
	const usage = `
feesim config

Show app config settings.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	result, err := c.Config()
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
}

func appMetrics(args []string, c *api.Client) {
	const usage = `
feesim metrics

Show app metrics.

`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}

	result, err := c.Metrics()
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(result, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
}
