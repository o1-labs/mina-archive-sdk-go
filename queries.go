package archive

// Hand-written GraphQL query strings. Each maps 1:1 to a method on Client.
// Field selections cover the full schema surface — callers wanting narrower
// selections can use Client.ExecuteQuery directly.

const queryEvents = `
query GetEvents($input: EventFilterOptionsInput!) {
  events(input: $input) {
    blockInfo {
      height
      stateHash
      parentHash
      ledgerHash
      chainStatus
      timestamp
      globalSlotSinceHardfork
      globalSlotSinceGenesis
      distanceFromMaxBlockHeight
    }
    eventData {
      accountUpdateId
      transactionInfo {
        status
        hash
        memo
        authorizationKind
        sequenceNumber
        zkappAccountUpdateIds
      }
      data
    }
  }
}
`

const queryActions = `
query GetActions($input: ActionFilterOptionsInput!) {
  actions(input: $input) {
    blockInfo {
      height
      stateHash
      parentHash
      ledgerHash
      chainStatus
      timestamp
      globalSlotSinceHardfork
      globalSlotSinceGenesis
      distanceFromMaxBlockHeight
    }
    transactionInfo {
      status
      hash
      memo
      authorizationKind
      sequenceNumber
      zkappAccountUpdateIds
    }
    actionData {
      accountUpdateId
      transactionInfo {
        status
        hash
        memo
        authorizationKind
        sequenceNumber
        zkappAccountUpdateIds
      }
      data
    }
    actionState {
      actionStateOne
      actionStateTwo
      actionStateThree
      actionStateFour
      actionStateFive
    }
  }
}
`

const queryNetworkState = `
query NetworkState {
  networkState {
    maxBlockHeight {
      canonicalMaxBlockHeight
      pendingMaxBlockHeight
    }
  }
}
`

const queryBlocks = `
query GetBlocks($query: BlockQueryInput, $limit: Int, $sortBy: BlockSortByInput) {
  blocks(query: $query, limit: $limit, sortBy: $sortBy) {
    blockHeight
    creator
    stateHash
    parentHash
    dateTime
    transactions {
      coinbase
      userCommands {
        hash
        kind
        from
        to
        amount
        fee
        memo
        nonce
        status
        failureReason
      }
      zkappCommands {
        hash
        feePayer
        fee
        memo
        status
        failureReason
      }
      feeTransfer {
        recipient
        fee
        type
      }
    }
  }
}
`
