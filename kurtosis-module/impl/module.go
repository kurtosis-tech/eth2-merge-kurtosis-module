package impl

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/forkmon"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/grafana"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/module_io"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/cl"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/cl/cl_client_rest_client"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/el"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prelaunch_data_generator"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prelaunch_data_generator/genesis_consts"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prometheus"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/static_files"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/testnet_verifier"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/transaction_spammer"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/enclaves"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
)

const (
	responseJsonLinePrefixStr = ""
	responseJsonLineIndentStr = "  "

	// TODO uncomment these when the module can either start a private network OR connect to an existing devnet
	// mergeDevnet3NetworkId = "1337602"
	// mergeDevnet3ClClientBootnodeEnr = "enr:-Iq4QKuNB_wHmWon7hv5HntHiSsyE1a6cUTK1aT7xDSU_hNTLW3R4mowUboCsqYoh1kN9v3ZoSu_WuvW9Aw0tQ0Dxv6GAXxQ7Nv5gmlkgnY0gmlwhLKAlv6Jc2VjcDI1NmsxoQK6S-Cii_KmfFdUJL2TANL3ksaKUnNXvTCv1tLwXs0QgIN1ZHCCIyk"

	// On mainnet, finalization will be head - 2
	// However, according to Pari, on these small testnets with genesis very close there's more churn so 4 epochs is possible
	firstHeadEpochWhereFinalizedEpochIsPossible = uint64(4)
	// The number of extra epochs beyond the first-epoch-where-finalization-is-possible that we'll wait for the network to finalize
	finalizedEpochTolerance = uint64(0)
	timeBetweenFinalizedEpochChecks = 5 * time.Second

	grafanaUser = "admin"
	grafanaPassword = "admin"
	grafanaDashboardPathUrl = "d/QdTOwy-nz/eth2-merge-kurtosis-module-dashboard?orgId=1"
)


type Eth2KurtosisModule struct {
}

func NewEth2KurtosisModule() *Eth2KurtosisModule {
	return &Eth2KurtosisModule{}
}

func (e Eth2KurtosisModule) Execute(enclaveCtx *enclaves.EnclaveContext, serializedParams string) (serializedResult string, resultError error) {
	logrus.Infof("Deserializing the following execute params:\n%v", serializedParams)
	paramsObj, err := module_io.DeserializeAndValidateParams(serializedParams)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred deserializing & validating the params")
	}
	networkParams := paramsObj.Network
	numParticipants := uint32(len(paramsObj.Participants))
	logrus.Info("Successfully deserialized execute params")

	// Parse templates early, so that any errors are caught before we do the stuff that takes a long time
	grafanaDatasourceConfigTemplate, err := static_files.ParseTemplate(static_files.GrafanaDatasourceConfigTemplateFilepath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred parsing Grafana datasource config template file '%v'", static_files.PrometheusConfigTemplateFilepath)
	}
	grafanaDashboardsConfigTemplate, err := static_files.ParseTemplate(static_files.GrafanaDashboardProvidersConfigTemplateFilepath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred parsing Grafana dashboards config template file '%v'", static_files.GrafanaDashboardProvidersConfigTemplateFilepath)
	}
	prometheusConfigTemplate, err := static_files.ParseTemplate(static_files.PrometheusConfigTemplateFilepath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred parsing prometheus config template file '%v'", static_files.PrometheusConfigTemplateFilepath)
	}

	logrus.Info("Creating prelaunch data generator...")
	prelaunchDataGeneratorCtx, err := prelaunch_data_generator.LaunchPrelaunchDataGenerator(
		enclaveCtx,
		networkParams.NetworkID,
		networkParams.DepositContractAddress,
		networkParams.TotalTerminalDifficulty,
		networkParams.PreregisteredValidatorKeysMnemonic,
	)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching the prelaunch data-generating container")
	}
	logrus.Info("Successfully created prelaunch data generator")

	logrus.Infof("Adding %v participants logging at level '%v'...", numParticipants, paramsObj.ClientLogLevel)
	participants, clGenesisUnixTimestamp, err := participant_network.LaunchParticipantNetwork(
		enclaveCtx,
		prelaunchDataGeneratorCtx,
		networkParams,
		paramsObj.Participants,
		paramsObj.ClientLogLevel,
		paramsObj.WaitForMining,
	)
	if err != nil {
		return "", stacktrace.Propagate(
			err,
			"An error occurred launching a participant network of '%v' participants",
			numParticipants,
		 )
	}
	allElClientContexts := []*el.ELClientContext{}
	allClClientContexts := []*cl.CLClientContext{}
	for _, participant := range participants {
		allElClientContexts = append(allElClientContexts, participant.GetELClientContext())
		allClClientContexts = append(allClClientContexts, participant.GetCLClientContext())
	}
	logrus.Infof("Successfully added %v participants", numParticipants)

	logrus.Info("Launching transaction spammer...")
	if err := transaction_spammer.LaunchTransanctionSpammer(
		enclaveCtx,
		genesis_consts.PrefundedAccounts,
		// TODO Upgrade the transaction spammer so it can take in multiple EL client addresses
		allElClientContexts[0],
	); err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching the transaction spammer")
	}
	logrus.Info("Successfully launched transaction spammer")


	if paramsObj.WaitForClGenesis {
		logrus.Info("Waiting until CL genesis occurs to add forkmon...")
		// We need to wait until the CL genesis has been reached to launch Forkmon because it has a bug (as of 2022-01-18) where
		//  if a CL ndoe's getHealth endpoint returns a non-200 error code, Forkmon will mark the node as failed and will never revisit it
		// This is fine with nodes who report 200 before genesis, but certain nodes (e.g. Lighthouse) will report a 503 before genesis
		// Therefore, the simple fix is wait until CL genesis to start Forkmon
		secondsRemainingUntilClGenesis := clGenesisUnixTimestamp - uint64(time.Now().Unix())
		if secondsRemainingUntilClGenesis < 0 {
			secondsRemainingUntilClGenesis = 0
		}
		durationUntilClGenesis := time.Duration(int64(secondsRemainingUntilClGenesis)) * time.Second
		time.Sleep(durationUntilClGenesis)
		logrus.Info("CL genesis has occurred")
	} else {
		logrus.Info("The wait-for-mining flag was set to false. Forkmon will be started immediately but may not work properly.")
	}


	logrus.Info("Launching forkmon...")
	forkmonConfigTemplate, err := static_files.ParseTemplate(static_files.ForkmonConfigTemplateFilepath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred parsing forkmon config template file '%v'", static_files.ForkmonConfigTemplateFilepath)
	}
	forkmonPublicUrl, err := forkmon.LaunchForkmon(
		enclaveCtx,
		forkmonConfigTemplate,
		allClClientContexts,
		clGenesisUnixTimestamp,
		networkParams.SecondsPerSlot,
		networkParams.SlotsPerEpoch,
	)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching forkmon service")
	}
	logrus.Infof("Successfully launched forkmon at '%v'", forkmonPublicUrl)

	logrus.Info("Launching prometheus...")
	prometheusPublicUrl, prometheusPrivateUrl, err := prometheus.LaunchPrometheus(
		enclaveCtx,
		prometheusConfigTemplate,
		allClClientContexts,
    )
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching prometheus service")
	}
	logrus.Infof("Successfully launched Prometheus at '%v'", prometheusPublicUrl)

	logrus.Info("Launching grafana...")
	grafanaPublicUrl, err := grafana.LaunchGrafana(
		enclaveCtx,
		grafanaDatasourceConfigTemplate,
		grafanaDashboardsConfigTemplate,
		prometheusPrivateUrl,
	)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred launching Grafana")
	}
	grafanaDashboardUrl := fmt.Sprintf("%v/%v", grafanaPublicUrl, grafanaDashboardPathUrl)
	logrus.Infof("Successfully launched Grafana at '%v'", grafanaPublicUrl)

	if paramsObj.WaitForVerifications {
		logrus.Info("Running synchronous testnet verification...")
		retCode, output, err := testnet_verifier.RunSynchronousTestnetVerification(paramsObj, enclaveCtx, allElClientContexts, allClClientContexts, networkParams.TotalTerminalDifficulty)
		if err != nil {
			return "", stacktrace.Propagate(err, "An error occurred running the merge testnet verification")
		}
		logrus.Info("Testnet verification has finished...")
		if retCode != 0 {
			logrus.Error("Some verifications were not successful")
			lines := strings.Split(output, "\n")
			for _, l := range lines {
				if strings.Contains(l, "lvl=crit") {
					logrus.Error(l)
				}
			}
			return "", fmt.Errorf("Some verifications were not successful")
		}
		logrus.Info("Successfully ran merge testnet verification, all verifications were successful")
	} else {

		logrus.Info("Launching asynchronous merge testnet verifier...")
		if err := testnet_verifier.LaunchAsynchronousTestnetVerifier(paramsObj, enclaveCtx, allElClientContexts, allClClientContexts, networkParams.TotalTerminalDifficulty); err != nil {
			return "", stacktrace.Propagate(err, "An error occurred launching the merge testnet verifier")
		}
		logrus.Info("Successfully launched merge testnet verifier")

		if paramsObj.WaitForFinalization {
			logrus.Info("Waiting for the first finalized epoch...")
			// TODO Make sure that ALL Beacon clients have finalized, not just the first one!!!
			firstClClientCtx := allClClientContexts[0]
			firstClClientRestClient := firstClClientCtx.GetRESTClient()
			if err := waitUntilFirstFinalizedEpoch(firstClClientRestClient, networkParams.SecondsPerSlot, networkParams.SlotsPerEpoch); err != nil {
				return "", stacktrace.Propagate(err, "An error occurred waiting until the first finalized epoch occurred")
			}
			logrus.Info("First finalized epoch occurred successfully")
		}

	}

	allClClientPeers := []string{}
	for _, clClientCtx := range allClClientContexts {
		allClClientPeers = append(allClClientPeers, fmt.Sprintf("/ip4/%s/tcp/%v/p2p/%s", clClientCtx.GetPublicIPAddress(), clClientCtx.GetPublicHTTPPortNum(), clClientCtx.GetPeerId()))
	}

	allElClientPeers := []string{}
	for _, elClientCtx := range allElClientContexts {
		internal := fmt.Sprintf("@%s:%v",elClientCtx.GetIPAddress(),elClientCtx.GetDiscoveryPortNum())
		external := fmt.Sprintf("@%s:%v",elClientCtx.GetPublicIPAddress(),elClientCtx.GetPublicDiscoveryPortNum())

		allElClientPeers = append(allElClientPeers, strings.Replace(elClientCtx.GetEnode(),internal,external,1))
	}

	responseObj := &module_io.ExecuteResponse{
		ForkmonPublicURL: forkmonPublicUrl,
		PrometheusPublicURL: prometheusPublicUrl,
		GrafanaInfo: &module_io.GrafanaInfo{
			PublicURL: grafanaPublicUrl,
			DashboardURL: grafanaDashboardUrl,
			User: grafanaUser,
			Password: grafanaPassword,
		},
		ClPeers: allClClientPeers,
		ElPeers: allElClientPeers,
	}
	responseStr, err := json.MarshalIndent(responseObj, responseJsonLinePrefixStr, responseJsonLineIndentStr)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred serializing the following response object to JSON for returning: %+v", responseObj)
	}

	return string(responseStr), nil
}


func waitUntilFirstFinalizedEpoch(
	restClient *cl_client_rest_client.CLClientRESTClient,
	secondsPerSlot uint32,
	slotsPerEpoch uint32,
) error {
	// If we wait long enough that we've just entered this epoch, we've waited too long - finality should already have happened
	waitedTooLongEpoch := firstHeadEpochWhereFinalizedEpochIsPossible + 1 + finalizedEpochTolerance
	timeoutSeconds := waitedTooLongEpoch * uint64(slotsPerEpoch) * uint64(secondsPerSlot)
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		currentSlot, err := restClient.GetCurrentSlot()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting the current slot using the REST client, which should never happen")
		}
		currentEpoch := currentSlot / uint64(slotsPerEpoch)
		finalizedEpoch, err := restClient.GetFinalizedEpoch()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred getting the finalized epoch using the REST client, which should never happen")
		}
		if finalizedEpoch > 0 {
			return nil
		}
		logrus.Debugf(
			"Finalized epoch hasn't occurred yet; current slot = '%v', current epoch = '%v', and finalized epoch = '%v'",
			currentSlot,
			currentEpoch,
			finalizedEpoch,
		 )
		time.Sleep(timeBetweenFinalizedEpochChecks)
	}
	return stacktrace.NewError("Waited for %v for a finalized epoch to occur, but it didn't happen", timeout)
}
