package cosmosclient

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/gogo/protobuf/proto"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/ignite/cli/ignite/pkg/cosmosfaucet"

	"github.com/evmos/ethermint/crypto/hd"
	ethcodec "github.com/evmos/evmos/v12/crypto/codec"
)

// FaucetTransferEnsureDuration is the duration that BroadcastTx will wait when a faucet transfer
// is triggered prior to broadcasting but transfer's tx is not committed in the state yet.
var FaucetTransferEnsureDuration = time.Second * 40

var errCannotRetrieveFundsFromFaucet = errors.New("cannot retrieve funds from faucet")

const (
	defaultNodeAddress   = "http://localhost:26657"
	defaultGasAdjustment = 1.0
	defaultGasLimit      = 300000

	defaultFaucetAddress   = "http://localhost:4500"
	defaultFaucetDenom     = "token"
	defaultFaucetMinAmount = 100

	defaultTXsPerPage = 30

	searchHeight = "tx.height"

	orderAsc = "asc"
)

type broadcastMode string

// Client is a client to access your chain by querying and broadcasting transactions.
type Client struct {
	// RPC is Tendermint RPC.
	RPC *HTTP

	// Factory is a Cosmos SDK tx factory.
	Factory Factory

	// context is a Cosmos SDK client context.
	context client.Context

	// AccountRegistry is the registry to access accounts.
	AccountRegistry cosmosaccount.Registry

	addressPrefix string

	nodeAddress string
	out         io.Writer
	chainID     string

	useFaucet       bool
	faucetAddress   string
	faucetDenom     string
	faucetMinAmount uint64

	homePath           string
	keyringServiceName string
	keyringBackend     cosmosaccount.KeyringBackend
	keyringDir         string

	gasLimit      uint64
	gas           string
	gasPrices     string
	fees          string
	gasAdjustment float64
	generateOnly  bool
	broadcastMode broadcastMode
}

// Option configures your client.
type Option func(*Client)

// WithHome sets the data dir of your chain. This option is used to access your chain's
// file based keyring which is only needed when you deal with creating and signing transactions.
// when it is not provided, your data dir will be assumed as `$HOME/.your-chain-id`.
func WithHome(path string) Option {
	return func(c *Client) {
		c.homePath = path
	}
}

// WithKeyringServiceName used as the keyring's name when you are using OS keyring backend.
// by default it is `cosmos`.
func WithKeyringServiceName(name string) Option {
	return func(c *Client) {
		c.keyringServiceName = name
	}
}

// WithKeyringBackend sets your keyring backend. By default, it is `test`.
func WithKeyringBackend(backend cosmosaccount.KeyringBackend) Option {
	return func(c *Client) {
		c.keyringBackend = backend
	}
}

// WithKeyringDir sets the directory of the keyring. By default, it uses cosmosaccount.KeyringHome
func WithKeyringDir(keyringDir string) Option {
	return func(c *Client) {
		c.keyringDir = keyringDir
	}
}

// WithNodeAddress sets the node address of your chain. When this option is not provided
// `http://localhost:26657` is used as default.
func WithNodeAddress(addr string) Option {
	return func(c *Client) {
		c.nodeAddress = addr
	}
}

// WithAddressPrefix sets the address prefix of your chain
func WithAddressPrefix(prefix string) Option {
	return func(c *Client) {
		c.addressPrefix = prefix
	}
}

// WithUseFaucet sets the faucet address the denom and amount
func WithUseFaucet(faucetAddress, denom string, minAmount uint64) Option {
	return func(c *Client) {
		c.useFaucet = true
		c.faucetAddress = faucetAddress
		if denom != "" {
			c.faucetDenom = denom
		}
		if minAmount != 0 {
			c.faucetMinAmount = minAmount
		}
	}
}

// WithGasLimit sets the gas limit
func WithGasLimit(gasLimit uint64) Option {
	return func(c *Client) {
		c.gasLimit = gasLimit
	}
}

// WithGas sets an explicit gas-limit on transactions.
// Set to "auto" to calculate automatically
func WithGas(gas string) Option {
	return func(c *Client) {
		c.gas = gas
	}
}

// WithGasPrices sets the gas prices
func WithGasPrices(gasPrices string) Option {
	return func(c *Client) {
		c.gasPrices = gasPrices
	}
}

// WithFees sets the fees (e.g. 10uatom)
func WithFees(fees string) Option {
	return func(c *Client) {
		c.fees = fees
	}
}

// WithGasAdjustment sets the gas adjustment (e.g. 1.3)
func WithGasAdjustment(gasAdj float64) Option {
	return func(c *Client) {
		c.gasAdjustment = gasAdj
	}
}

// WithBroadcastMode sets the broadcast mode
func WithBroadcastMode(broadcastMode broadcastMode) Option {
	return func(c *Client) {
		c.broadcastMode = broadcastMode
	}
}

// New creates a new client with given options.
func New(ctx context.Context, options ...Option) (Client, error) {
	c := Client{
		nodeAddress:     defaultNodeAddress,
		keyringBackend:  cosmosaccount.KeyringTest,
		addressPrefix:   "cosmos",
		faucetAddress:   defaultFaucetAddress,
		faucetDenom:     defaultFaucetDenom,
		faucetMinAmount: defaultFaucetMinAmount,
		out:             io.Discard,
		gas:             strconv.Itoa(defaultGasLimit),
		gasAdjustment:   defaultGasAdjustment,
	}

	var err error

	for _, apply := range options {
		apply(&c)
	}

	if c.RPC, err = NewHttp(c.nodeAddress, "/websocket"); err != nil {
		return Client{}, err
	}

	statusResp, err := c.RPC.Status(ctx)
	if err != nil {
		return Client{}, err
	}

	c.chainID = statusResp.NodeInfo.Network

	if c.homePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Client{}, err
		}
		c.homePath = filepath.Join(home, "."+c.chainID)
	}

	if c.keyringDir == "" {
		c.keyringDir = c.homePath
	}

	c.AccountRegistry, err = cosmosaccount.New(
		cosmosaccount.WithKeyringServiceName(c.keyringServiceName),
		cosmosaccount.WithKeyringBackend(c.keyringBackend),
		cosmosaccount.WithHome(c.keyringDir),
	)
	if err != nil {
		return Client{}, err
	}

	//Overwrite the keyring with EthSecp256k1 supported keyring
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	// same in both
	ethcodec.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)
	customKeyring, err := keyring.New(c.keyringServiceName, string(c.keyringBackend), c.homePath, os.Stdin, cdc, hd.EthSecp256k1Option())
	if err != nil {
		return Client{}, err
	}

	c.AccountRegistry.Keyring = customKeyring
	c.context = newContext(c).WithKeyring(customKeyring)
	c.Factory = newFactory(c.context, c)

	return c, nil
}

// Account returns the account with name or address equal to nameOrAddress.
func (c Client) Account(nameOrAddress string) (cosmosaccount.Account, error) {
	// defer c.lockBech32Prefix()()

	acc, err := c.AccountRegistry.GetByName(nameOrAddress)
	if err == nil {
		return acc, nil
	}
	return c.AccountRegistry.GetByAddress(nameOrAddress)
}

// Address returns the account address from account name.
func (c Client) Address(accountName string) (sdktypes.AccAddress, error) {
	account, err := c.Account(accountName)
	if err != nil {
		return sdktypes.AccAddress{}, err
	}
	return account.Record.GetAddress()
}

// Context returns the client context.
func (c Client) Context() client.Context {
	return c.context
}

// SetConfigAddressPrefix sets the account prefix in the SDK global config
// func (c Client) SetConfigAddressPrefix() {
// 	// TODO find a better way if possible.
// 	// https://github.com/ignite/cli/issues/2744
// 	mconf.Lock()
// 	defer mconf.Unlock()
// 	config := sdktypes.GetConfig()
// 	config.SetBech32PrefixForAccount(c.addressPrefix, c.addressPrefix+"pub")
// }

// Response of your broadcasted transaction.
type Response struct {
	Codec codec.Codec

	// TxResponse is the underlying tx response.
	*sdktypes.TxResponse
}

// Decode decodes the proto func response defined in your Msg service into your message type.
// message needs to be a pointer. and you need to provide the correct proto message(struct) type to the Decode func.
//
// e.g., for the following CreateChain func the type would be: `types.MsgCreateChainResponse`.
//
// ```proto
//
//	service Msg {
//	  rpc CreateChain(MsgCreateChain) returns (MsgCreateChainResponse);
//	}
//
// ```
func (r Response) Decode(message proto.Message) error {
	data, err := hex.DecodeString(r.Data)
	if err != nil {
		return err
	}

	var txMsgData sdktypes.TxMsgData
	if err := r.Codec.Unmarshal(data, &txMsgData); err != nil {
		return err
	}

	// check deprecated Data
	if len(txMsgData.Data) != 0 {
		resData := txMsgData.Data[0]
		return prototypes.UnmarshalAny(&prototypes.Any{
			// TODO get type url dynamically(basically remove `+ "Response"`) after the following issue has solved.
			// https://github.com/ignite/cli/issues/2098
			// https://github.com/cosmos/cosmos-sdk/issues/10496
			TypeUrl: resData.MsgType + "Response",
			Value:   resData.Data,
		}, message)
	}

	resData := txMsgData.MsgResponses[0]
	return prototypes.UnmarshalAny(&prototypes.Any{
		TypeUrl: resData.TypeUrl,
		Value:   resData.Value,
	}, message)
}

// Status returns the node status
func (c Client) Status(ctx context.Context) (*ctypes.ResultStatus, error) {
	return c.RPC.Status(ctx)
}

// BroadcastTx creates and broadcasts a tx with given messages for account.
func (c Client) BroadcastTx(accountName string, msgs ...sdktypes.Msg) (Response, error) {
	_, broadcast, err := c.BroadcastTxWithProvision(accountName, msgs...)
	if err != nil {
		return Response{}, err
	}
	return broadcast()
}

// protects sdktypes.Config.
var mconf sync.Mutex

// BroadcastTxWithProvision creates and broadcasts a tx with given messages for account.
func (c Client) BroadcastTxWithProvision(accountName string, msgs ...sdktypes.Msg) (
	gas uint64, broadcast func() (Response, error), err error) {
	if err := c.prepareBroadcast(context.Background(), accountName, msgs); err != nil {
		return 0, nil, err
	}

	// TODO find a better way if possible.
	mconf.Lock()
	defer mconf.Unlock()

	accountAddress, err := c.Address(accountName)
	if err != nil {
		return 0, nil, err
	}

	ctx := c.context.
		WithFromName(accountName).
		WithFromAddress(accountAddress)

	txf, err := prepareFactory(ctx, c.Factory)
	if err != nil {
		return 0, nil, err
	}

	_, gas, err = CalculateGas(ctx, txf, msgs...)
	if err != nil {
		return 0, nil, err
	}
	// the simulated gas can vary from the actual gas needed for a real transaction
	// we add an additional amount to endure sufficient gas is provided
	gas += 10000
	txf = txf.WithGas(gas)

	// Return the provision function
	return gas, func() (Response, error) {
		txUnsigned, err := txf.BuildUnsignedTx(msgs...)
		if err != nil {
			return Response{}, err
		}

		txUnsigned.SetFeeGranter(ctx.GetFeeGranterAddress())
		if err := Sign(txf, accountName, txUnsigned, true); err != nil {
			return Response{}, err
		}

		txBytes, err := ctx.TxConfig.TxEncoder()(txUnsigned.GetTx())
		if err != nil {
			return Response{}, err
		}

		resp, err := ctx.BroadcastTx(txBytes)
		if err == sdkerrors.ErrInsufficientFunds {
			err = c.makeSureAccountHasTokens(context.Background(), accountAddress.String())
			if err != nil {
				return Response{}, err
			}
			resp, err = ctx.BroadcastTx(txBytes)
		}

		return Response{
			Codec:      ctx.Codec,
			TxResponse: resp,
		}, handleBroadcastResult(resp, err)
	}, nil
}

// prepareBroadcast performs checks and operations before broadcasting messages
func (c *Client) prepareBroadcast(ctx context.Context, accountName string, _ []sdktypes.Msg) error {
	// TODO uncomment after https://github.com/tendermint/spn/issues/363
	// validate msgs.
	//  for _, msg := range msgs {
	//  if err := msg.ValidateBasic(); err != nil {
	//  return err
	//  }
	//  }

	accountAddress, err := c.Address(accountName)
	if err != nil {
		return err
	}

	// make sure that account has enough balances before broadcasting.
	if c.useFaucet {
		if err := c.makeSureAccountHasTokens(ctx, accountAddress.String()); err != nil {
			return err
		}
	}

	return nil
}

// makeSureAccountHasTokens makes sure the address has a positive balance
// it requests funds from the faucet if the address has an empty balance
func (c *Client) makeSureAccountHasTokens(ctx context.Context, address string) error {
	if err := c.checkAccountBalance(ctx, address); err == nil {
		return nil
	}

	// request coins from the faucet.
	fc := cosmosfaucet.NewClient(c.faucetAddress)
	faucetResp, err := fc.Transfer(ctx, cosmosfaucet.TransferRequest{AccountAddress: address})
	if err != nil {
		return errors.Wrap(errCannotRetrieveFundsFromFaucet, err.Error())
	}
	if faucetResp.Error != "" {
		return errors.Wrap(errCannotRetrieveFundsFromFaucet, faucetResp.Error)
	}

	// make sure funds are retrieved.
	ctx, cancel := context.WithTimeout(ctx, FaucetTransferEnsureDuration)
	defer cancel()

	return backoff.Retry(func() error {
		return c.checkAccountBalance(ctx, address)
	}, backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx))
}

func (c *Client) checkAccountBalance(ctx context.Context, address string) error {
	resp, err := banktypes.NewQueryClient(c.context).Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   c.faucetDenom,
	})
	if err != nil {
		return err
	}

	if resp.Balance.Amount.Uint64() >= c.faucetMinAmount {
		return nil
	}

	return fmt.Errorf("account has not enough %q balance, min. required amount: %d", c.faucetDenom, c.faucetMinAmount)
}

// handleBroadcastResult handles the result of broadcast messages result and checks if an error occurred
func handleBroadcastResult(resp *sdktypes.TxResponse, err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.New("make sure that your SPN account has enough balance")
		}

		return err
	}

	if resp.Code > 0 {
		return fmt.Errorf("SPN error with '%d' code: %s", resp.Code, resp.RawLog)
	}
	return nil
}

func prepareFactory(clientCtx client.Context, txf Factory) (Factory, error) {
	from := clientCtx.GetFromAddress()

	if err := txf.AccountRetriever().EnsureExists(clientCtx, from); err != nil {
		return txf, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
		if err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	return txf, nil
}

func newContext(c Client) client.Context {
	var (
		amino             = codec.NewLegacyAmino()
		interfaceRegistry = codectypes.NewInterfaceRegistry()
		marshaler         = codec.NewProtoCodec(interfaceRegistry)
		txConfig          = authtx.NewTxConfig(marshaler, authtx.DefaultSignModes)
	)

	//Register ethermint interfaces
	//ethcodec.RegisterLegacyAminoCodec(amino)
	ethcodec.RegisterInterfaces(interfaceRegistry)

	authtypes.RegisterInterfaces(interfaceRegistry)
	//cryptocodec.RegisterInterfaces(interfaceRegistry)
	sdktypes.RegisterInterfaces(interfaceRegistry)
	staking.RegisterInterfaces(interfaceRegistry)
	//cryptocodec.RegisterInterfaces(interfaceRegistry)

	return client.Context{}.
		WithChainID(c.chainID).
		WithInterfaceRegistry(interfaceRegistry).
		WithCodec(marshaler).
		WithTxConfig(txConfig).
		WithLegacyAmino(amino).
		WithInput(os.Stdin).
		WithOutput(c.out).
		WithAccountRetriever(AccountRetriever{addressPrefix: c.addressPrefix}).
		WithBroadcastMode(string(c.broadcastMode)).
		WithHomeDir(c.homePath).
		WithClient(c.RPC).
		WithSkipConfirmation(true)
}

func newFactory(clientCtx client.Context, client Client) Factory {
	return Factory{}.
		WithChainID(clientCtx.ChainID).
		WithKeybase(clientCtx.Keyring).
		WithGas(client.gasLimit).
		WithGasPrices(client.gasPrices).
		WithFees(client.fees).
		WithGasAdjustment(client.gasAdjustment).
		WithSignMode(signing.SignMode_SIGN_MODE_UNSPECIFIED).
		WithAccountRetriever(clientCtx.AccountRetriever).
		WithTxConfig(clientCtx.TxConfig)
}
