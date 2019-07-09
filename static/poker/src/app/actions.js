/** @format */

const {ADD, REMOVE, FOLD, CALL, BET, ALLIN} = require('../lib/types')

// const POST_URL = "http://localhost:5000/ln-pkr/us-central1/action";
const POST_URL = 'https://us-central1-ln-pkr.cloudfunctions.net/action'

const dispatch = args =>
  window
    .fetch(POST_URL, {
      method: 'POST',
      body: JSON.stringify(args),
      headers: {'Content-Type': 'application/json'}
    })
    .then(raw => raw.json())

const addPlayer = (tableId, accountId, position) => {
  // add checks for
  // accountId, tableId, position, chips...

  return dispatch({
    type: ADD,
    accountId,
    tableId,
    position
  })
}

const removePlayer = (tableId, accountId, playerId) =>
  dispatch({
    type: REMOVE,
    accountId,
    playerId,
    tableId
  })

const call = (tableId, playerId) =>
  dispatch({
    type: CALL,
    tableId,
    playerId
  })

const fold = (tableId, playerId) => {
  return dispatch({
    type: FOLD,
    tableId,
    playerId
  })
}

const bet = (tableId, playerId, amount) => {
  return dispatch({
    type: BET,
    tableId,
    playerId,
    amount
  })
}

const allin = (tableId, playerId) => {
  return dispatch({
    type: ALLIN,
    tableId,
    playerId
  })
}

export {addPlayer, removePlayer, call, fold, bet, allin}
