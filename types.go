package archive

import "encoding/json"

// BlockStatusFilter filters events/actions by consensus status.
type BlockStatusFilter string

const (
	// BlockStatusAll matches all blocks (canonical + pending).
	BlockStatusAll BlockStatusFilter = "ALL"
	// BlockStatusPending matches only pending (not-yet-finalized) blocks.
	BlockStatusPending BlockStatusFilter = "PENDING"
	// BlockStatusCanonical matches only canonical (finalized) blocks.
	BlockStatusCanonical BlockStatusFilter = "CANONICAL"
)

// BlockSortBy controls the sort order of GetBlocks results.
type BlockSortBy string

const (
	// BlockSortAsc sorts blocks by ascending block height.
	BlockSortAsc BlockSortBy = "BLOCKHEIGHT_ASC"
	// BlockSortDesc sorts blocks by descending block height.
	BlockSortDesc BlockSortBy = "BLOCKHEIGHT_DESC"
)

// EventFilterOptionsInput filters events from a specific account.
//
// Use zero/nil values to omit a field — only Address is required.
type EventFilterOptionsInput struct {
	Address string            // required
	TokenID string            // optional
	Status  BlockStatusFilter // optional ("" = server default)
	From    *int              // optional, inclusive
	To      *int              // optional, exclusive
}

// ActionFilterOptionsInput filters actions from a specific account.
type ActionFilterOptionsInput struct {
	Address         string
	TokenID         string
	Status          BlockStatusFilter
	From            *int
	To              *int
	FromActionState string
	EndActionState  string
}

// VerificationKeyUpdateFilterInput filters applied account updates that set a
// verification key.
//
// Unlike the event and action filters, the block range is required: the server
// bounds the span by its configured BLOCK_RANGE_SIZE. From is inclusive and To
// is exclusive.
type VerificationKeyUpdateFilterInput struct {
	VerificationKeyHash string            // required
	From                int               // required, inclusive
	To                  int               // required, exclusive
	Status              BlockStatusFilter // optional ("" = server default)
}

// BlockQueryInput filters blocks by height, date, or canonical status.
type BlockQueryInput struct {
	BlockHeightGte *int   // inclusive
	BlockHeightLt  *int   // exclusive
	DateTimeGte    string // ISO-8601
	DateTimeLt     string // ISO-8601
	Canonical      *bool
	InBestChain    *bool
}

// VerificationKeyUpdate is an applied account update that set a verification key.
type VerificationKeyUpdate struct {
	AccountUpdateID string `json:"accountUpdateId"`
	// Address is the account whose verification key was set.
	Address             string          `json:"address"`
	TokenID             string          `json:"tokenId"`
	VerificationKeyHash string          `json:"verificationKeyHash"`
	BlockInfo           BlockInfo       `json:"blockInfo"`
	TransactionInfo     TransactionInfo `json:"transactionInfo"`
}

// TransactionInfo describes the transaction that emitted an event/action.
type TransactionInfo struct {
	Status                string `json:"status"`
	Hash                  string `json:"hash"`
	Memo                  string `json:"memo"`
	AuthorizationKind     string `json:"authorizationKind"`
	SequenceNumber        int    `json:"sequenceNumber"`
	ZkappAccountUpdateIDs []int  `json:"zkappAccountUpdateIds"`
}

// EventData is one event record from the archive.
type EventData struct {
	AccountUpdateID string           `json:"accountUpdateId"`
	TransactionInfo *TransactionInfo `json:"transactionInfo"`
	Data            []string         `json:"data"`
}

// ActionData is one action record from the archive.
type ActionData struct {
	AccountUpdateID string           `json:"accountUpdateId"`
	TransactionInfo *TransactionInfo `json:"transactionInfo"`
	Data            []string         `json:"data"`
}

// BlockInfo carries the block-level metadata returned alongside an
// event/action group.
type BlockInfo struct {
	Height                     int    `json:"height"`
	StateHash                  string `json:"stateHash"`
	ParentHash                 string `json:"parentHash"`
	LedgerHash                 string `json:"ledgerHash"`
	ChainStatus                string `json:"chainStatus"`
	Timestamp                  string `json:"timestamp"`
	GlobalSlotSinceHardfork    int    `json:"globalSlotSinceHardfork"`
	GlobalSlotSinceGenesis     int    `json:"globalSlotSinceGenesis"`
	DistanceFromMaxBlockHeight int    `json:"distanceFromMaxBlockHeight"`
}

// ActionStates is the actionState quintuple attached to each ActionOutput.
type ActionStates struct {
	ActionStateOne   *string `json:"actionStateOne"`
	ActionStateTwo   *string `json:"actionStateTwo"`
	ActionStateThree *string `json:"actionStateThree"`
	ActionStateFour  *string `json:"actionStateFour"`
	ActionStateFive  *string `json:"actionStateFive"`
}

// EventOutput is one entry returned from GetEvents.
type EventOutput struct {
	BlockInfo *BlockInfo   `json:"blockInfo"`
	EventData []*EventData `json:"eventData"`
}

// ActionOutput is one entry returned from GetActions.
type ActionOutput struct {
	BlockInfo       *BlockInfo       `json:"blockInfo"`
	TransactionInfo *TransactionInfo `json:"transactionInfo"`
	ActionData      []*ActionData    `json:"actionData"`
	ActionState     ActionStates     `json:"actionState"`
}

// MaxBlockHeightInfo is the archive node's high-water mark.
type MaxBlockHeightInfo struct {
	CanonicalMaxBlockHeight int `json:"canonicalMaxBlockHeight"`
	PendingMaxBlockHeight   int `json:"pendingMaxBlockHeight"`
}

// NetworkStateOutput is returned from GetNetworkState.
type NetworkStateOutput struct {
	MaxBlockHeight *MaxBlockHeightInfo `json:"maxBlockHeight"`
}

// UserCommand is a non-zkApp transaction inside a block.
//
// Amount and Fee are nanomina decimal strings; parse with CurrencyFromGraphQL.
type UserCommand struct {
	Hash          string `json:"hash"`
	Kind          string `json:"kind"`
	From          string `json:"from"`
	To            string `json:"to"`
	Amount        string `json:"amount"`
	Fee           string `json:"fee"`
	Memo          string `json:"memo"`
	Nonce         int    `json:"nonce"`
	Status        string `json:"status"`
	FailureReason string `json:"failureReason"`
}

// ZkAppCommand is a zkApp transaction inside a block.
type ZkAppCommand struct {
	Hash          string `json:"hash"`
	FeePayer      string `json:"feePayer"`
	Fee           string `json:"fee"`
	Memo          string `json:"memo"`
	Status        string `json:"status"`
	FailureReason string `json:"failureReason"`
}

// FeeTransfer is a fee-transfer transaction inside a block.
type FeeTransfer struct {
	Recipient string `json:"recipient"`
	Fee       string `json:"fee"`
	Type      string `json:"type"`
}

// BlockTransactions bundles the four transaction kinds in a block.
type BlockTransactions struct {
	Coinbase      string         `json:"coinbase"`
	UserCommands  []UserCommand  `json:"userCommands"`
	ZkappCommands []ZkAppCommand `json:"zkappCommands"`
	FeeTransfer   []FeeTransfer  `json:"feeTransfer"`
}

// Block is one entry returned from GetBlocks.
type Block struct {
	BlockHeight  int               `json:"blockHeight"`
	Creator      string            `json:"creator"`
	StateHash    string            `json:"stateHash"`
	ParentHash   string            `json:"parentHash"`
	DateTime     string            `json:"dateTime"`
	Transactions BlockTransactions `json:"transactions"`
}

// toEventInput converts the public input struct into the GraphQL JSON shape.
// Empty/zero fields are omitted so the server applies its defaults.
func (in EventFilterOptionsInput) toMap() map[string]any {
	m := map[string]any{"address": in.Address}
	if in.TokenID != "" {
		m["tokenId"] = in.TokenID
	}
	if in.Status != "" {
		m["status"] = string(in.Status)
	}
	if in.From != nil {
		m["from"] = *in.From
	}
	if in.To != nil {
		m["to"] = *in.To
	}
	return m
}

func (in ActionFilterOptionsInput) toMap() map[string]any {
	m := map[string]any{"address": in.Address}
	if in.TokenID != "" {
		m["tokenId"] = in.TokenID
	}
	if in.Status != "" {
		m["status"] = string(in.Status)
	}
	if in.From != nil {
		m["from"] = *in.From
	}
	if in.To != nil {
		m["to"] = *in.To
	}
	if in.FromActionState != "" {
		m["fromActionState"] = in.FromActionState
	}
	if in.EndActionState != "" {
		m["endActionState"] = in.EndActionState
	}
	return m
}

func (in VerificationKeyUpdateFilterInput) toMap() map[string]any {
	m := map[string]any{
		"verificationKeyHash": in.VerificationKeyHash,
		"from":                in.From,
		"to":                  in.To,
	}
	if in.Status != "" {
		m["status"] = string(in.Status)
	}
	return m
}

func (in BlockQueryInput) toMap() map[string]any {
	m := map[string]any{}
	if in.BlockHeightGte != nil {
		m["blockHeight_gte"] = *in.BlockHeightGte
	}
	if in.BlockHeightLt != nil {
		m["blockHeight_lt"] = *in.BlockHeightLt
	}
	if in.DateTimeGte != "" {
		m["dateTime_gte"] = in.DateTimeGte
	}
	if in.DateTimeLt != "" {
		m["dateTime_lt"] = in.DateTimeLt
	}
	if in.Canonical != nil {
		m["canonical"] = *in.Canonical
	}
	if in.InBestChain != nil {
		m["inBestChain"] = *in.InBestChain
	}
	return m
}

// graphqlRequest / graphqlResponse are wire-level wrappers.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage     `json:"data"`
	Errors []GraphQLErrorEntry `json:"errors"`
}
