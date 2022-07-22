# TBD
### Features
* Added CircleCi `check_latest_version` workflow for running a scheduled pipeline every day to control successful module execution

### Fixes
* Fixed broken link in the readme file

# 0.5.6
### Features
* EL mining waiter now logs failures to mine as a debug message
* Removes miner on Besu
* Adds static peers for Erigon and Nethermind

# 0.5.5
### Changes
* Migrate to using internal cli tool, `kudet`, for new release workflow and getting docker image tags
* Upgrade core to 1.55.2
* Upgrade module-api-lib to 0.17.0

# 0.5.4
* Geth: open up vhost/cors configs by adding relevant flags. explicitly set empty bootnode for first node.
* Changed consensus layer fork versions to not conflict with the Prater testnet configuration
* Add support for YAML in input serialized params

# 0.5.3
### Changes
* Upgraded to module-api-lib 0.16.0, core 1.54.1 and engine 1.26.1 for latest Kurtosis compatibility
* Upgraded Ubuntu machine image in Circle CI configuration to version `ubuntu-2004:202201-02`

# 0.5.2
* Update besu params
* Update default images
* Update Nimbus to use HTTP instead of WS

# 0.5.1
### Fixes
* Fix one last listening-on-private-IP

# 0.5.0
### Breaking Changes
* `ExecuteResponse` no longer returns public URLs for services in the module

# 0.4.21
### Fixes
* Fixed metrics listeners erroneously listening on their private IP address rather than 0.0.0.0

# 0.4.20
### Changes
* Switch service port IDs to be Kubernetes-friendly

# 0.4.19
### Features
* Lighthouse images are run with `RUST_BACKTRACE=full`

### Changes
* Upgraded to module-api-lib 0.15.0 for latest Kurtosis compatibility

# 0.4.18
### Features
* Added support for Erigon as an EL client option (`elType: "erigon"`)
  * Default Erigon docker image (`elImage` module option) is set to [thorax/erigon:devel](https://hub.docker.com/layers/erigon/thorax/erigon/devel/images/sha256-8d1c07fb8b88f8bde6ca2a2d42ff0e0cb0206a0009dacbf9b3571721aaa921d7)

### Changes
* Switched the default `clImage` for Lighthouse client to `sigp/lighthouse:latest` from `sigp/lighthouse:latest-unstable`

# 0.4.17
### Changes
* Upgraded to module-api-lib 0.14.1, switching to the files API for moving files between containers

### Fixes
* Corrected strange indentation in README

# 0.4.16
* Updates prysm config
* Increases nimbus timeout
* Updates default images

# 0.4.15
* Added `Merge-Testnet-Verifier` module and wait-for-verifications flag

# 0.4.14
### Fixes
* Modify `ElClientLauncher` interface to take in multiple EL client contexts, as a hackaround for a bug in Nethermind peering

# 0.4.13
### Fixes
* Limit the number of Nimbus threads to 4 so that Nimbus won't crash in the cases where the host has > 255 threads

# 0.4.12
### Fixes
* Add a `0x` prefix to the JWT token contents, since Nimbus won't accept JWT tokens without it

# 0.4.11
### Features
* Added a `Developing` section to the docs
* Added a link in the README to the source code where default params are defined
* EL & CL genesis generation now creates a JWT key
* Lighthouse, Teku, and Geth, Nethermind, Besu now consume the JWT key

# 0.4.10
### Fixes
* Fixed metrics config in Teku CL container config

### Features
* Prysm can now be a boot node, as https://github.com/kurtosis-tech/eth2-merge-kurtosis-module/issues/37 seems to be fixed from the Prysm side

# 0.4.9
### Features
* Print an extra log message when the wait-for-mining flag is set to false
* Log the module's parameters it receives for debugging purposes

### Fixes
* Fixed an issue where Lodestar wasn't properly using the `clImage` param

# 0.4.8
### Features
* Adding participation panel to grafana dashboard

# 0.4.7
### Features
* The CI job will now `enclave dump` its results for debugging purposes
* Added extra links in the README to give users extra information on running the module

### Changes
* The default client when no parameters are supplied is now Lighthouse (was Nimbus)

### Changes
* Adds `subscribe` to `nethermind`
* Changes wait times for `geth`
* Adds EL flag for `prysm`

# 0.4.6
###Features
* Add Prometheus with a Grafana dashboard to show the network's state

# 0.4.5
### Changes
* Suggest users store their module parameters in a file, so they're easier to work with

### Fixes
* Fix port IDs for Kurtosis 0.10.0

# 0.4.4
### Changes
* Upped the Lodestar wait-for-availability time to 60s
* Updated flags for Kiln testnet compatibility
* Switches key generation to use insecure mode, making key generation extremely fast

# 0.4.3
### Features
* Add extra debug logging to EL REST client, for debugging any issues
* Add new module's params `elLogLevel` and `clLogLevel` to configure a specific EL and CL client's log level
* Added the `elExtraParams`, `beaconExtraParams`, and `validatorExtraParams` properties to a participant to allow for overriding participant commands

### Changes
* Set config values to `BELLATRIX_` rather than `MERGE_`
* Set `ETH1_FOLLOW_DISTANCE` to 30

### Fixes
* Use emptystring for Besu ENR, as there's no way to get it right now without the logs
* Fixed a Teku break caused by a flag getting renamed in the latest version of the Teku image

# 0.4.2
### Features
* Added generation of Besu genesis file
* Added Besu EL

### Fixes
* Fixed an issue where the CL REST client would try to deserialize the bodies of responses that came back with non-200 status codes
* When a Teku node is present, require merge fork epoch to be >= 3 as a workaround for a bug in Teku
* Disallow a Prysm node being a boot node due to https://github.com/kurtosis-tech/eth2-merge-kurtosis-module/pull/36

### Changes
* Set the `mergeForkBlock` parameter in the EL genesis config template to `10` per Pari's recommendation
* Switch back to [the default genesis generator](https://github.com/skylenet/ethereum-genesis-generator) (rather than the Kurtosis fork of it)
* Nethermind genesis JSON is generated using the genesis generator image
* Centralized EL client availability-waiter and mining-waiter logic

# 0.4.1
### Fixes
* Fixed an issue where using emptystring as the default image wasn't working

# 0.4.0
### Features
* Allowed configurable EL & CL client images via the `elImage` and `clImage` keys to participant object
* Support `trace` loglevel

### Breaking Changes
* The participant spec's `el` and `cl` keys have been switched to `elType` and `clType`

# 0.3.0
### Features
* Added Prysm CL (beacon, validator) Launcher
* Made EL & CL client log levels configurable as a module param, `logLevel`
* Added self-documenting code for module params
* When an invalid EL or CL client type is provided in the params, the valid values are printed to the user
* Added a `waitForMining` property to the config, to allow users to skip the EL client mine-waiting (only useful for debugging a CL client)

### Changes
* The `WaitForBeaconClientAvailability` method also checks if the returned status is READY, which means the node is synced
* Replaced the custom implementation of the availability waiter method in Lodestar Launcher with the `WaitForBeaconClientAvailability` used for other launchers
* Set the Eth1 block time to 1 second in the CL config
* Revert back to the original Lodestar image, and comment out the `BELLATRIX_` config values for now

### Fixes
* Set the `--subscribe-all-subnets` flag equivalents on all Beacon nodes
* Generate the CL genesis files AFTER the EL network is mining, so that the CL network doesn't skip any important epochs (e.g. Altair, or merge fork) which causes it to get in a stuck state
* Removed unneeded hanging-around delay that existed in wait-for-finalization logic
* Wait until all CL nodes are up before starting to process slots
* Make forkmon respond to slots-per-epoch config changes
* Bump Lodestar wait-for-availability time up to 30s
* Don't launch Forkmon until CL genesis has been hit, due to a bug where if it receives a non-200 healthcheck status for a node then it won't ever revisit the node
* Updated the Geth image to `parithoshj/geth:merge-f72c361` (from around 2022-01-18)
* Updated TODOs in README

# 0.2.3
### Features
* Added the ability to specify arbitrary numbers of participants with EL/CL combos, and default to one Geth+Nimbus participant
* Added instructions to the README for configuring the module

### Fixes
* Get rid of the 300-second delay in the generated CL genesis
* Added the Lighthouse validator node

# 0.2.2
### Features
* Added a functioning Nimbus CL client
* Added Nethermind EL client
* Added a Lodestar CL (beacon, validator) Launcher
* * Added new `GetNodeSyncingData` method in cl rest client
* Added a transaction spammer to blast the network with transactions after all the nodes come up
* Added optional waiting until epoch finalization occurs

# 0.2.1
* Empty commit to try and kick CircleCI into actually building the tag

# 0.2.0
### Features
* Add build infra
* The Geth + Lighthouse node inside of Kurtosis now syncs with merge-devnet-3!
* Successfully-working private, mining network!
* Added a network of consensus-layer clients
* Hooked up genesis generation for Geth & CL nodes
* Lighthouse nodes peer with each other
* Add forkmon to the started network
* Add Teku CL client
* Enable CI

### Fixes
* Correct merge parameters like TTD, Altair fork version, merge fork version, etc. per Parithosh's recommendations
* Give Teku nodes 120s to start

### Changes
* Refactor the structure to reflect that there should be one EL node per CL node (and prepare for separated Beacon/validator nodes, like Lighthouse does)

# 0.1.0
* Initial commit
