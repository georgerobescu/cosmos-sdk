package tx

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/rest"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
)

// QueryTxCmd implements the default command for a tx query.
func QueryTxCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx [hash]",
		Short: "Matches this txhash over all committed blocks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// find the key to look up the account
			hashHexStr := args[0]

			cliCtx := context.NewCLIContext().WithCodec(cdc)

			output, err := queryTx(cdc, cliCtx, hashHexStr)
			if err != nil {
				return err
			}

			fmt.Println(string(output))
			return nil
		},
	}

	cmd.Flags().StringP(client.FlagNode, "n", "tcp://localhost:26657", "Node to connect to")
	viper.BindPFlag(client.FlagNode, cmd.Flags().Lookup(client.FlagNode))
	cmd.Flags().Bool(client.FlagTrustNode, false, "Trust connected full node (don't verify proofs for responses)")
	viper.BindPFlag(client.FlagTrustNode, cmd.Flags().Lookup(client.FlagTrustNode))
	return cmd
}

func queryTx(cdc *codec.Codec, cliCtx context.CLIContext, hashHexStr string) ([]byte, error) {
	hash, err := hex.DecodeString(hashHexStr)
	if err != nil {
		return nil, err
	}

	node, err := cliCtx.GetNode()
	if err != nil {
		return nil, err
	}

	res, err := node.Tx(hash, !cliCtx.TrustNode)
	if err != nil {
		return nil, err
	}

	if !cliCtx.TrustNode {
		err := ValidateTxResult(cliCtx, res)
		if err != nil {
			return nil, err
		}
	}

	info, err := formatTxResult(cdc, res)
	if err != nil {
		return nil, err
	}

	if cliCtx.Indent {
		return cdc.MarshalJSONIndent(info, "", "  ")
	}
	return cdc.MarshalJSON(info)
}

// ValidateTxResult performs transaction verification
func ValidateTxResult(cliCtx context.CLIContext, res *ctypes.ResultTx) error {
	check, err := cliCtx.Verify(res.Height)
	if err != nil {
		return err
	}

	err = res.Proof.Validate(check.Header.DataHash)
	if err != nil {
		return err
	}
	return nil
}

func formatTxResult(cdc *codec.Codec, res *ctypes.ResultTx) (sdk.TxResponse, error) {
	tx, err := parseTx(cdc, res.Tx)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	return sdk.NewResponseResultTx(res, tx), nil
}

func parseTx(cdc *codec.Codec, txBytes []byte) (sdk.Tx, error) {
	var tx auth.StdTx

	err := cdc.UnmarshalBinaryLengthPrefixed(txBytes, &tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// REST

// transaction query REST handler
func QueryTxRequestHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		hashHexStr := vars["hash"]

		output, err := queryTx(cdc, cliCtx, hashHexStr)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cdc, output, cliCtx.Indent)
	}
}
