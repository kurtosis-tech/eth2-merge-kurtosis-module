package besu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/module_io"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/participant_network/el"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/prelaunch_data_generator/genesis_consts"
	"github.com/kurtosis-tech/eth2-merge-kurtosis-module/kurtosis-module/impl/service_launch_utils"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis-core-api-lib/api/golang/lib/services"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	rpcPortNum       uint16 = 8545
	wsPortNum        uint16 = 8546
	discoveryPortNum uint16 = 30303

	// Port IDs
	rpcPortId          = "rpc"
	wsPortId           = "ws"
	tcpDiscoveryPortId = "tcp-discovery"
	udpDiscoveryPortId = "udp-discovery"

	// NOTE: This can't be 0x00000....000
	// See: https://github.com/ethereum/go-ethereum/issues/19547
	miningRewardsAccount = "0x0000000000000000000000000000000000000001"

	// The filepath of the genesis JSON file in the shared directory, relative to the shared directory root
	sharedGenesisJsonRelFilepath = "genesis.json"

	// The dirpath of the execution data directory on the client container
	executionDataDirpathOnClientContainer = "/opt/besu/execution-data"
	keystoreDirpathOnClientContainer = executionDataDirpathOnClientContainer + "/keystore"

	gethKeysRelDirpathInSharedDir = "geth-keys"

	jsonContentTypeHeader = "application/json"
	rpcRequestTimeout = 5 * time.Second

	getNodeInfoRpcRequestBody = `{"jsonrpc":"2.0","method": "admin_nodeInfo","params":[],"id":1}`

	expectedSecondsForBesuInit = 5
	expectedSecondsPerKeyImport = 8
	expectedSecondsAfterNodeStartUntilHttpServerIsAvailable = 10
	getNodeInfoTimeBetweenRetries = 1 * time.Second

	gethAccountPassword      = "password"          // Password that the Besu accounts will be locked with
	gethAccountPasswordsFile = "/tmp/password.txt" // Importing an account to
)
var usedPorts = map[string]*services.PortSpec{
	rpcPortId:          services.NewPortSpec(rpcPortNum, services.PortProtocol_TCP),
	wsPortId:           services.NewPortSpec(wsPortNum, services.PortProtocol_TCP),
	tcpDiscoveryPortId: services.NewPortSpec(discoveryPortNum, services.PortProtocol_TCP),
	// TODO Remove if there's no UDP discovery port?????
	udpDiscoveryPortId: services.NewPortSpec(discoveryPortNum, services.PortProtocol_UDP),
}
var entrypointArgs = []string{"sh", "-c"}
var besuLogLevels = map[module_io.ParticipantLogLevel]string{
	module_io.ParticipantLogLevel_Error: "ERROR",
	module_io.ParticipantLogLevel_Warn:  "WARN",
	module_io.ParticipantLogLevel_Info:  "INFO",
	module_io.ParticipantLogLevel_Debug: "DEBUG",
	module_io.ParticipantLogLevel_Trace: "TRACE",
}

type BesuELClientLauncher struct {
	genesisJsonFilepathOnModuleContainer string
	prefundedAccountInfo []*genesis_consts.PrefundedAccount
	networkId string
}

func NewBesuELClientLauncher(genesisJsonFilepathOnModuleContainer string, prefundedAccountInfo []*genesis_consts.PrefundedAccount, networkId string) *BesuELClientLauncher {
	return &BesuELClientLauncher{genesisJsonFilepathOnModuleContainer: genesisJsonFilepathOnModuleContainer, prefundedAccountInfo: prefundedAccountInfo, networkId: networkId}
}

func (launcher *BesuELClientLauncher) Launch(
	enclaveCtx *enclaves.EnclaveContext,
	serviceId services.ServiceID,
	image string,
	logLevel module_io.ParticipantLogLevel,
	bootnodeContext *el.ELClientContext,
) (resultClientCtx *el.ELClientContext, resultErr error) {
	containerConfigSupplier := launcher.getContainerConfigSupplier(image, launcher.networkId, bootnodeContext, logLevel)
	serviceCtx, err := enclaveCtx.AddService(serviceId, containerConfigSupplier)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred launching the Besu EL client with service ID '%v'", serviceId)
	}

	nodeInfo, err := launcher.getNodeInfoWithRetry(serviceCtx.GetPrivateIPAddress())
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the newly-started node's info")
	}

	miningWaiter := newBesuMiningWaiter(
		serviceCtx.GetPrivateIPAddress(),
		rpcPortNum,
	)
	result := el.NewELClientContext(
		nodeInfo.ENR,
		nodeInfo.Enode,
		serviceCtx.GetPrivateIPAddress(),
		rpcPortNum,
		wsPortNum,
		miningWaiter,
	)

	return result, nil
}


// ====================================================================================================
//                                       Private Helper Methods
// ====================================================================================================
func (launcher *BesuELClientLauncher) getContainerConfigSupplier(
	image string,
	networkId string,
	bootnodeContext *el.ELClientContext, // NOTE: If this is empty, the node will be configured as a bootnode
	logLevel module_io.ParticipantLogLevel,
) func(string, *services.SharedPath) (*services.ContainerConfig, error) {
	result := func(privateIpAddr string, sharedDir *services.SharedPath) (*services.ContainerConfig, error) {
		besuLogLevel, found := besuLogLevels[logLevel]
		if !found {
			return nil, stacktrace.NewError("No Besu log level was defined for client log level '%v'; this is a bug in this module itself", logLevel)
		}

		genesisJsonSharedPath := sharedDir.GetChildPath(sharedGenesisJsonRelFilepath)
		if err := service_launch_utils.CopyFileToSharedPath(launcher.genesisJsonFilepathOnModuleContainer, genesisJsonSharedPath); err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred copying genesis JSON file '%v' into shared directory path '%v'", launcher.genesisJsonFilepathOnModuleContainer, sharedGenesisJsonRelFilepath)
		}

		// TODO MODIFY FOR BESU
		/*
		gethKeysDirSharedPath := sharedDir.GetChildPath(gethKeysRelDirpathInSharedDir)
		if err := os.Mkdir(gethKeysDirSharedPath.GetAbsPathOnThisContainer(), os.ModePerm); err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred creating the Besu keys directory in the shared dir")
		}


		accountAddressesToUnlock := []string{}
		for _, prefundedAccount := range launcher.prefundedAccountInfo {
			// TODO MAYBE NEED TO MAKE THIS BESU-SPECIFIC??
			keyFilepathOnModuleContainer := prefundedAccount.GethKeyFilepath
			keyFilename := path.Base(keyFilepathOnModuleContainer)
			keyRelFilepathInSharedDir := path.Join(gethKeysRelDirpathInSharedDir, keyFilename)
			keyFileSharedPath := sharedDir.GetChildPath(keyRelFilepathInSharedDir)
			if err := service_launch_utils.CopyFileToSharedPath(keyFilepathOnModuleContainer, keyFileSharedPath); err != nil {
				return nil, stacktrace.Propagate(err, "An error occurred copying key file '%v' to the shared directory", keyFilepathOnModuleContainer)
			}

			accountAddressesToUnlock = append(accountAddressesToUnlock, prefundedAccount.Address)
		}

		 */

		// TODO FIGURE OUT IF WE NEED TO DO AN INIT
		/*
		initDatadirCmdStr := fmt.Sprintf(
			"geth init --datadir=%v %v",
			executionDataDirpathOnClientContainer,
			genesisJsonSharedPath.GetAbsPathOnServiceContainer(),
		)

		copyKeysIntoKeystoreCmdStr := fmt.Sprintf(
			"cp -r %v/* %v/",
			gethKeysDirSharedPath.GetAbsPathOnServiceContainer(),
			keystoreDirpathOnClientContainer,
		)

		createPasswordsFileCmdStr := fmt.Sprintf(
			"{ for i in $(seq 1 %v); do echo \"%v\" >> %v; done; }",
			len(launcher.prefundedAccountInfo),
			gethAccountPassword,
			gethAccountPasswordsFile,
		)

		 */

		// TODO ADD UNLOCKED ACCOUNTS
		// accountsToUnlockStr := strings.Join(accountAddressesToUnlock, ",")
		launchNodeCmdArgs := []string{
			"besu",
			"--logging=" + besuLogLevel,
			"--data-path=" + executionDataDirpathOnClientContainer,
			"--genesis-file=" + genesisJsonSharedPath.GetAbsPathOnServiceContainer(),
			"--network-id=" + networkId,
			"--host-allowlist=*",
			"--Xmerge-support=true",
			"--miner-enabled=true",
			"--miner-coinbase=" + miningRewardsAccount,
			"--rpc-http-enabled=true",
			"--rpc-http-host=0.0.0.0",
			fmt.Sprintf("--rpc-http-port=%v", rpcPortNum),
			"--rpc-http-api=ADMIN,CLIQUE,MINER,ETH,NET,DEBUG,TXPOOL,EXECUTION",
			"--rpc-http-cors-origins=*",
			"--rpc-ws-enabled=true",
			"--rpc-ws-host=0.0.0.0",
			fmt.Sprintf("--rpc-ws-port=%v", wsPortNum),
			"--rpc-ws-api=ADMIN,CLIQUE,MINER,ETH,NET,DEBUG,TXPOOL,EXECUTION",
			"--p2p-enabled=true",
			"--p2p-host=" + privateIpAddr,
			fmt.Sprintf("--p2p-port=%v", discoveryPortNum),
		}
		if bootnodeContext != nil {
			launchNodeCmdArgs = append(
				launchNodeCmdArgs,
				"--bootnodes=" + bootnodeContext.GetEnode(),
			)
		}
		launchNodeCmdStr := strings.Join(launchNodeCmdArgs, " ")

		// TODO FIX OR DELETE
		/*
		subcommandStrs := []string{
			initDatadirCmdStr,
			copyKeysIntoKeystoreCmdStr,
			createPasswordsFileCmdStr,
			launchNodeCmdStr,
		}
		commandStr := strings.Join(subcommandStrs, " && ")

		 */

		containerConfig := services.NewContainerConfigBuilder(
			image,
		).WithUsedPorts(
			usedPorts,
		).WithEntrypointOverride(
			entrypointArgs,
		).WithCmdOverride([]string{
			launchNodeCmdStr,
			// commandStr,
		}).Build()

		return containerConfig, nil
	}
	return result
}

// TODO Extract into a common location!!!
func (launcher *BesuELClientLauncher) getNodeInfoWithRetry(privateIpAddr string) (NodeInfo, error) {
	maxNumRetries := expectedSecondsForBesuInit + len(launcher.prefundedAccountInfo) * expectedSecondsPerKeyImport + expectedSecondsAfterNodeStartUntilHttpServerIsAvailable

	getNodeInfoResponse := new(GetNodeInfoResponse)
	for i := 0; i < maxNumRetries; i++ {
		if err := sendRpcCall(privateIpAddr, getNodeInfoRpcRequestBody, getNodeInfoResponse); err == nil {
			return getNodeInfoResponse.Result, nil
		} else {
			logrus.Debugf("Getting the node info via RPC failed with error: %v", err)
		}
		time.Sleep(getNodeInfoTimeBetweenRetries)
	}
	return NodeInfo{}, stacktrace.NewError("Couldn't get the node's info even after %v retries with %v between retries", maxNumRetries, getNodeInfoTimeBetweenRetries)
}

func sendRpcCall(privateIpAddr string, requestBody string, targetStruct interface{}) error {
	url := fmt.Sprintf("http://%v:%v", privateIpAddr, rpcPortNum)
	var jsonByteArray = []byte(requestBody)

	logrus.Debugf("Sending RPC call to '%v' with JSON body '%v'...", url, requestBody)

	client := http.Client{
		Timeout: rpcRequestTimeout,
	}
	resp, err := client.Post(url, jsonContentTypeHeader, bytes.NewBuffer(jsonByteArray))
	if err != nil {
		return stacktrace.Propagate(err, "Failed to send RPC request to Besu node with private IP '%v'", privateIpAddr)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return stacktrace.NewError(
			"Received non-%v status code '%v' on RPC request to Besu node with private IP '%v'",
			http.StatusOK,
			resp.StatusCode,
			privateIpAddr,
		)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return stacktrace.Propagate(err, "Error reading the RPC call response body")
	}
	bodyString := string(bodyBytes)
	logrus.Tracef("Response for RPC call %v: %v", requestBody, bodyString)

	json.Unmarshal(bodyBytes, targetStruct)
	if err := json.Unmarshal(bodyBytes, targetStruct); err != nil {
		return stacktrace.Propagate(err, "Error JSON-parsing Besu node RPC response string '%v' into a struct", bodyString)
	}
	return nil
}