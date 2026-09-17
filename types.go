package archive

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

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
//
// The date bounds must be ISO-8601. The server coerces them with JavaScript's
// new Date(value).getTime(); a value it cannot parse becomes NaN, which reaches
// SQL as the literal string "NaN" and matches nothing *without erroring* — the
// query returns HTTP 200 and an empty list. "14/08/2023" fails this way, and
// "Aug 14 2023" parses but depends on the server's local timezone.
//
// Use SetDateTimeRange or DateTimeFilter rather than formatting by hand.
type BlockQueryInput struct {
	BlockHeightGte *int // inclusive
	BlockHeightLt  *int // exclusive
	// DateTimeGte is the inclusive lower bound, ISO-8601. See the type doc for
	// the silent-empty-result hazard; prefer SetDateTimeRange.
	DateTimeGte string
	// DateTimeLt is the exclusive upper bound, ISO-8601. See the type doc for
	// the silent-empty-result hazard; prefer SetDateTimeRange.
	DateTimeLt  string
	Canonical   *bool
	InBestChain *bool
}

// Time parses Timestamp, which is Unix epoch milliseconds as a decimal string.
//
// This exists because the obvious call is wrong in two different ways:
// time.Parse(time.RFC3339, bi.Timestamp) returns a parse error, and treating
// the value as seconds silently yields a date in 1970.
func (bi BlockInfo) Time() (time.Time, error) {
	ms, err := strconv.ParseInt(bi.Timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("BlockInfo.Timestamp %q is not Unix epoch milliseconds: %w", bi.Timestamp, err)
	}
	return time.UnixMilli(ms).UTC(), nil
}

// Time parses DateTime, which is ISO-8601.
//
// Note this is a different encoding from BlockInfo.Timestamp, which is Unix
// epoch milliseconds as a decimal string.
func (b Block) Time() (time.Time, error) {
	t, err := time.Parse(time.RFC3339, b.DateTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("Block.DateTime %q is not ISO-8601: %w", b.DateTime, err)
	}
	return t.UTC(), nil
}

// DateTimeFilter formats t for the DateTimeGte/DateTimeLt bounds.
//
// The result is always a value the server parses to a finite number, so it
// cannot produce the silent empty result a hand-written string can.
func DateTimeFilter(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// SetDateTimeRange sets both date bounds from time.Time values, lower bound
// inclusive and upper bound exclusive, matching the server's range semantics.
// A zero time.Time leaves that bound unset.
func (in *BlockQueryInput) SetDateTimeRange(gte, lt time.Time) {
	if !gte.IsZero() {
		in.DateTimeGte = DateTimeFilter(gte)
	}
	if !lt.IsZero() {
		in.DateTimeLt = DateTimeFilter(lt)
	}
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
	Status            string `json:"status"`
	Hash              string `json:"hash"`
	Memo              string `json:"memo"`
	AuthorizationKind string `json:"authorizationKind"`
	SequenceNumber    int    `json:"sequenceNumber"`
	// ZkappAccountUpdateIDs is element-nullable in the SDL ([Int]!), so a
	// member may be nil even though the list itself is always present. A value
	// type here would decode a null to 0, indistinguishable from id 0.
	ZkappAccountUpdateIDs []*int `json:"zkappAccountUpdateIds"`
}

// EventData is one event record from the archive.
type EventData struct {
	AccountUpdateID string           `json:"accountUpdateId"`
	TransactionInfo *TransactionInfo `json:"transactionInfo"`
	// Data is element-nullable in the SDL ([String]!), so a member may be nil
	// even though the list itself is always present. A value type here would
	// decode a null to "", indistinguishable from an empty string.
	Data []*string `json:"data"`
}

// ActionData is one action record from the archive.
type ActionData struct {
	AccountUpdateID string           `json:"accountUpdateId"`
	TransactionInfo *TransactionInfo `json:"transactionInfo"`
	// Data is element-nullable in the SDL ([String]!), so a member may be nil
	// even though the list itself is always present. A value type here would
	// decode a null to "", indistinguishable from an empty string.
	Data []*string `json:"data"`
}

// BlockInfo carries the block-level metadata returned alongside an
// event/action group.
type BlockInfo struct {
	Height      int    `json:"height"`
	StateHash   string `json:"stateHash"`
	ParentHash  string `json:"parentHash"`
	LedgerHash  string `json:"ledgerHash"`
	ChainStatus string `json:"chainStatus"`
	// Timestamp is Unix epoch MILLISECONDS as a decimal string, e.g.
	// "1692054601000" — not RFC3339. Parsing it with time.RFC3339 fails, and
	// reading it as seconds puts the block in 1970. Use BlockInfo.Time().
	// Contrast Block.DateTime, which is ISO-8601.
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
	Hash   string `json:"hash"`
	Kind   string `json:"kind"`
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
	Memo   string `json:"memo"`
	Nonce  int    `json:"nonce"`
	Status string `json:"status"`
	// FailureReason is nullable in the SDL. nil means the command did not
	// fail; a non-nil empty string means it failed with an empty reason.
	FailureReason *string `json:"failureReason"`
}

// ZkAppCommand is a zkApp transaction inside a block.
type ZkAppCommand struct {
	Hash     string `json:"hash"`
	FeePayer string `json:"feePayer"`
	Fee      string `json:"fee"`
	Memo     string `json:"memo"`
	Status   string `json:"status"`
	// FailureReason is nullable in the SDL. nil means the command did not
	// fail; a non-nil empty string means it failed with an empty reason.
	FailureReason *string `json:"failureReason"`
}

// FeeTransfer is a fee-transfer transaction inside a block.
type FeeTransfer struct {
	Recipient string `json:"recipient"`
	Fee       string `json:"fee"`
	Type      string `json:"type"`
}

// BlockTransactions bundles the four transaction kinds in a block.
type BlockTransactions struct {
	// Coinbase is populated regardless of ENABLE_BLOCK_TRANSACTION_DETAILS.
	Coinbase string `json:"coinbase"`
	// UserCommands is empty unless the server sets ENABLE_BLOCK_TRANSACTION_DETAILS=true.
	UserCommands []UserCommand `json:"userCommands"`
	// ZkappCommands is empty unless the server sets ENABLE_BLOCK_TRANSACTION_DETAILS=true.
	ZkappCommands []ZkAppCommand `json:"zkappCommands"`
	// FeeTransfer is empty unless the server sets ENABLE_BLOCK_TRANSACTION_DETAILS=true.
	FeeTransfer []FeeTransfer `json:"feeTransfer"`
}

// Block is one entry returned from GetBlocks.
//
// Transaction detail is gated behind the server's
// ENABLE_BLOCK_TRANSACTION_DETAILS, which defaults to false. Against a stock
// server ParentHash is "" and Transactions.UserCommands,
// Transactions.ZkappCommands and Transactions.FeeTransfer are all empty, while
// Transactions.Coinbase is populated — so the response looks healthy and is
// easily mistaken for an empty chain or an SDK bug.
type Block struct {
	BlockHeight int    `json:"blockHeight"`
	Creator     string `json:"creator"`
	StateHash   string `json:"stateHash"`
	// ParentHash is "" unless the server sets ENABLE_BLOCK_TRANSACTION_DETAILS=true.
	ParentHash string `json:"parentHash"`
	// DateTime is an ISO-8601 instant, e.g. "2023-08-14T23:10:01.000Z". The
	// server derives it from the same archive column that BlockInfo.Timestamp
	// exposes raw, so the two carry the same kind of value in two different
	// encodings. Use Block.Time().
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
