# Feesim
Feesim is a Bitcoin fee estimation program. Unlike fee estimation in Bitcoin
Core, Feesim makes use of the current mempool state and transaction arrival
rate, allowing it to be responsive to traffic surges. The fee estimates can be
seen [here](https://bitcoinfees.github.io).

## Model-based fee estimation
Fee estimation in Feesim is model-based; estimates are obtained from Monte Carlo
simulation of a queueing model of the bitcoin network. In essence, we find the
solution to the question: "If we assume that miners prioritize transactions by
fee rate, subject to a max block size and min fee rate, and that transactions
arrive at the same rate as the past hour, then given the current mempool size,
what fee rate is required for a transaction to be confirmed within X blocks
(with success probability P)?"

This allows the estimation algorithm to accommodate variations in network
conditions. For example, if a block hasn't been found in an hour and there's a
large mempool backlog, the algorithm should recognize that and raise the fee
estimates. Alternately, if there are daily lulls in transaction arrival rate,
the fee estimates should reflect that as well, allowing the user to pay lower
fees during the lull periods.

### Model parameter estimation
Feesim collects Bitcoin network data through the Bitcoin Core JSON-RPC API, and
computes estimates for:
* The network hashrate
* The distribution of max block size and min fee rate
* Short-term transaction arrival rate / joint distribution of `(txFeerate,
  txSize)`

Together with the current mempool state, these are used to perform the
simulation and obtain the fee estimates.

### Model validation
During normal operation, Feesim will predict the confirmation time of each
transaction and keep a tally of the proportion of transactions which met the
prediction. This proportion should be close to the success probability (default
90%), if the model is accurate. These scores can be be seen
[here](https://bitcoinfees.github.io/misc/predictscores).

## Running Feesim
### Installation
Install from source using at least Go 1.6:
```sh
$ go get github.com/bitcoinfees/feesim
```
Feesim uses the Go 1.5 vendor experiment, so alternatively you can install with
Go 1.5 by setting the environment variable `GO15VENDOREXPERIMENT=1`.

### Running
Feesim requires JSON-RPC access to a Bitcoin Core node (which can be pruned).
The RPC settings should be specified in `config.yml`, as such:
```yml
bitcoinrpc:
    username: myusername
    password: mypassword
    # host: myhost # Required if not localhost
    # port: myport # Required if not 8332
```
The config file should be placed in the data directory:
* Linux: `~/.feesim`
* OS X: `~/Library/Application Support/Feesim`
* Windows: `%LOCALAPPDATA%\Feesim`

Upon running `feesim start`, the program will start data collection, and then
begin running the simulation once there is sufficient data. It needs to be
online all the time, as it collects mempool data which cannot be obtained by
offline analysis.

`feesim status` shows the program status:
```sh
$ feesim status
result      : Tx estimation window size was 0s, should be at least 600s
txsource    : Tx estimation window size was 0s, should be at least 600s
blocksource : Block coverage was only 0/2016, should be at least 1008/2016.
mempool     : OK
```
`result` shows whether or not fee estimates are available. By default, fee
estimates require at least 10 minutes of transaction data, and data from 1008 of
the last 2016 blocks.

Once there is sufficient data, the simulation will begin to run and produce fee
estimates. The interface mirrors that of `bitcoin-cli estimatefee`:
```sh
$ feesim estimatefee 1
0.00030112
```
This shows the minimum fee rate for a transaction to be confirmed in 1 block,
with 90% probability (configurable).

Unlike `bitcoin-cli`, if the input argument is ommitted or is 0, the estimates
for all confirmation times is returned:
```sh
$ feesim estimatefee
 1: 0.00030138
 2: 0.00026738
 3: 0.00020492
 4: 0.00015988
 5: 0.00012805
 6: 0.00011478
 7: 0.00010616
 8: 0.00010001
 9: 0.00007519
10: 0.00005020
11: 0.00005000
12: 0.00005000
```
The underlying JSON-RPC API is compatible with Bitcoin Core's, so Feesim can be
used as drop-in replacement for the `estimatefee` API:
```sh
$ bitcoin-cli -rpcport=8350 estimatefee 1
0.00030138
```

### Bitcoin Core minrelaytxfee

Feesim will not produce fee estimates that are lower than the `minrelaytxfee` of
the Bitcoin Core node you are connecting to; you should thus set `minrelaytxfee`
accordingly. Also, if you change `minrelaytxfee`, you will need to restart
Feesim so that the changes can be registered.

### CPU considerations
The simulation is CPU intensive, whereas data collection is not, so you may not
want to run the sim all the time, while still collecting data. To do this, use
`feesim pause` to pause the simulation, and `feesim unpause` to resume.

By default, fee estimates are updated every minute. It's possible, however, that
a single simulation run takes longer than a minute, due to insufficient CPU
resources or exceptionally high transaction traffic. In general, this will not
cause any major problems; it only causes the fee estimates to be updated less
regularly. It's possible, nevertheless, to reduce the simulation run time by
lowering `maxblocksconfirms` or `numiters` in the config.

You can monitor the simulation run time with `feesim metrics`; `sim.X` are the
run time statistics, in nanoseconds, for roughly the last `X` simulation runs.

### Configuration

Please see `config.yml` in this repository for an example config file.

### Bootstrapping

As mentioned earlier, Feesim requires, by default, data from 1008 of the past
2016 blocks (it must be online when the blocks are discovered in order for the
data to be logged). This is about 1 week; if you don't want to wait that long,
you can contact me for a copy of the block data.
