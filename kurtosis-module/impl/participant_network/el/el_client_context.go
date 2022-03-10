package el

type ELClientContext struct {
	enr           string
	enode         string
	ipAddr        string
	rpcPortNum    uint16
	wsPortNum     uint16
	enginePortNum uint16
	miningWaiter  ELClientMiningWaiter
}

func NewELClientContext(enr string, enode string, ipAddr string, rpcPortNum uint16, wsPortNum uint16, enginePortNum uint16, miningWaiter ELClientMiningWaiter) *ELClientContext {
	return &ELClientContext{enr: enr, enode: enode, ipAddr: ipAddr, rpcPortNum: rpcPortNum, wsPortNum: wsPortNum, enginePortNum: enginePortNum, miningWaiter: miningWaiter}
}

func (ctx *ELClientContext) GetENR() string {
	return ctx.enr
}
func (ctx *ELClientContext) GetEnode() string {
	return ctx.enode
}
func (ctx *ELClientContext) GetIPAddress() string {
	return ctx.ipAddr
}
func (ctx *ELClientContext) GetRPCPortNum() uint16 {
	return ctx.rpcPortNum
}
func (ctx *ELClientContext) GetWSPortNum() uint16 {
	return ctx.wsPortNum
}

func (ctx *ELClientContext) GetEnginePortNum() uint16 {
	return ctx.enginePortNum
}

func (ctx *ELClientContext) GetMiningWaiter() ELClientMiningWaiter {
	return ctx.miningWaiter
}
