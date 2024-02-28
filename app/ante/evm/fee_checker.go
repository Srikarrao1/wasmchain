package evm

import (
	"math"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	//"github.com/cosmos/cosmos-sdk/types/address"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	anteutils "github.com/anryton/anryton/v2/app/ante/utils"
	anrytontypes "github.com/anryton/anryton/v2/types"
	"github.com/anryton/anryton/v2/x/evm/types"
)

var isGenesistxn = true

type GetValidator struct {
	stakingKeeper    anteutils.StakingKeeper
	dynamicfeeKeeper anteutils.DynamicFeeEVMKeeper
}

func NewDecorator(
	sk anteutils.StakingKeeper,
	dk anteutils.DynamicFeeEVMKeeper,
) GetValidator {
	return GetValidator{
		stakingKeeper:    sk,
		dynamicfeeKeeper: dk,
	}
}

func (sk GetValidator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// feeTx, ok := tx.(sdk.FeeTx)
	// if !ok {
	// 	return ctx, errorsmod.Wrap(errortypes.ErrTxDecode, "Tx must be a FeeTx")
	// }

	var (
		priority int64
	)
	// feePayer := feeTx.FeePayer()
	// _, check := sk.stakingKeeper.GetValidator(ctx, feePayer.Bytes())
	// delegator := sk.stakingKeeper.GetDelegatorDelegations(ctx, feePayer.Bytes(), 10)

	// params := sk.dynamicfeeKeeper.GetParams(ctx)
	// denom := params.EvmDenom
	// // gas := feeTx.GetGas()
	// feeCoins := feeTx.GetFee()
	// fee := feeCoins.AmountOfNoDenomValidation(denom)
	// // feeCap := fee.Quo(sdkmath.NewIntFromUint64(gas))

	// if isGenesistxn == false {
	// 	if check == false && (len(delegator) == 0) && fee.IsZero() {
	// 		// If the fee payer is neither a delegator nor a validator and the fees are zero, return an error
	// 		return ctx, errorsmod.Wrapf(errortypes.ErrInsufficientFee, "gas prices too low, got: required:. Please retry using a higher gas price or a higher fee")
	// 	}
	// }

	// isGenesistxn = false

	newCtx := ctx.WithPriority(priority)
	return next(newCtx, tx, simulate)

}

// NewDynamicFeeChecker returns a `TxFeeChecker` that applies a dynamic fee to
// Cosmos txs using the EIP-1559 fee market logic.
// This can be called in both CheckTx and deliverTx modes.
// a) feeCap = tx.fees / tx.gas
// b) tipFeeCap = tx.MaxPriorityPrice (default) or MaxInt64
// - when `ExtensionOptionDynamicFeeTx` is omitted, `tipFeeCap` defaults to `MaxInt64`.
// - when london hardfork is not enabled, it falls back to SDK default behavior (validator min-gas-prices).
// - Tx priority is set to `effectiveGasPrice / DefaultPriorityReduction`.
func NewDynamicFeeChecker(k DynamicFeeEVMKeeper) anteutils.TxFeeChecker {
	return func(ctx sdk.Context, feeTx sdk.FeeTx) (sdk.Coins, int64, error) {

		if ctx.BlockHeight() == 0 {
			// genesis transactions: fallback to min-gas-price logic
			return checkTxFeeWithValidatorMinGasPrices(ctx, feeTx)
		}

		params := k.GetParams(ctx)
		denom := params.EvmDenom
		ethCfg := params.ChainConfig.EthereumConfig(k.ChainID())

		baseFee := k.GetBaseFee(ctx, ethCfg)
		if baseFee == nil {
			// london hardfork is not enabled: fallback to min-gas-prices logic
			return checkTxFeeWithValidatorMinGasPrices(ctx, feeTx)
		}

		// default to `MaxInt64` when there's no extension option.
		maxPriorityPrice := sdkmath.NewInt(math.MaxInt64)

		// get the priority tip cap from the extension option.
		if hasExtOptsTx, ok := feeTx.(authante.HasExtensionOptionsTx); ok {
			for _, opt := range hasExtOptsTx.GetExtensionOptions() {
				if extOpt, ok := opt.GetCachedValue().(*anrytontypes.ExtensionOptionDynamicFeeTx); ok {
					maxPriorityPrice = extOpt.MaxPriorityPrice
					break
				}
			}
		}

		// priority fee cannot be negative
		if maxPriorityPrice.IsNegative() {
			return nil, 0, errorsmod.Wrapf(errortypes.ErrInsufficientFee, "max priority price cannot be negative")
		}

		gas := feeTx.GetGas()
		feeCoins := feeTx.GetFee()
		fee := feeCoins.AmountOfNoDenomValidation(denom)

		feeCap := fee.Quo(sdkmath.NewIntFromUint64(gas))

		baseFeeInt := sdkmath.NewIntFromBigInt(baseFee)

		// if feeCap.LT(baseFeeInt) {
		// 	return nil, 0, errorsmod.Wrapf(errortypes.ErrInsufficientFee, "gas prices too low, got: %s%s required: %s%s. Please retry using a higher gas price or a higher fee", feeCap, denom, baseFeeInt, denom)
		// }

		// calculate the effective gas price using the EIP-1559 logic.
		effectivePrice := sdkmath.NewIntFromBigInt(types.EffectiveGasPrice(baseFeeInt.BigInt(), feeCap.BigInt(), maxPriorityPrice.BigInt()))

		// NOTE: create a new coins slice without having to validate the denom
		effectiveFee := sdk.Coins{
			{
				Denom:  denom,
				Amount: effectivePrice.Mul(sdkmath.NewIntFromUint64(gas)),
			},
		}

		bigPriority := effectivePrice.Sub(baseFeeInt).Quo(types.DefaultPriorityReduction)
		priority := int64(math.MaxInt64)

		if bigPriority.IsInt64() {
			priority = bigPriority.Int64()
		}

		return effectiveFee, priority, nil
	}
}

// checkTxFeeWithValidatorMinGasPrices implements the default fee logic, where the minimum price per
// unit of gas is fixed and set by each validator, and the tx priority is computed from the gas price.
func checkTxFeeWithValidatorMinGasPrices(ctx sdk.Context, tx sdk.FeeTx) (sdk.Coins, int64, error) {

	feeCoins := tx.GetFee()
	minGasPrices := ctx.MinGasPrices()
	gas := int64(tx.GetGas()) //#nosec G701 -- checked for int overflow on ValidateBasic()

	// Ensure that the provided fees meet a minimum threshold for the validator,
	// if this is a CheckTx. This is only for local mempool purposes, and thus
	// is only ran on check tx.
	if ctx.IsCheckTx() && !minGasPrices.IsZero() {
		requiredFees := make(sdk.Coins, len(minGasPrices))

		// Determine the required fees by multiplying each required minimum gas
		// price by the gas limit, where fee = ceil(minGasPrice * gasLimit).
		glDec := sdk.NewDec(gas)
		for i, gp := range minGasPrices {
			fee := gp.Amount.Mul(glDec)
			requiredFees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
		}

		if !feeCoins.IsAnyGTE(requiredFees) {
			return nil, 0, errorsmod.Wrapf(errortypes.ErrInsufficientFee, "insufficient fees; got: %s required: %s", feeCoins, requiredFees)
		}
	}

	priority := getTxPriority(feeCoins, gas)
	return feeCoins, priority, nil
}

// getTxPriority returns a naive tx priority based on the amount of the smallest denomination of the gas price
// provided in a transaction.
func getTxPriority(fees sdk.Coins, gas int64) int64 {
	var priority int64

	for _, fee := range fees {
		gasPrice := fee.Amount.QuoRaw(gas)
		amt := gasPrice.Quo(types.DefaultPriorityReduction)
		p := int64(math.MaxInt64)

		if amt.IsInt64() {
			p = amt.Int64()
		}

		if priority == 0 || p < priority {
			priority = p
		}
	}

	return priority
}
