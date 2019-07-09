/** @format */

// poker state machine
// changes all players and the table itself

// process is like this !!!
// 1. process talked, clear talked state (important)
// 2. check for end of round
// 3. move active to the next player

// "check," which is to not place a bet, or "open,"
// which is to make the first bet.

// After the first bet each player may "fold,"
// which is to drop out of the hand losing any bets they have already made

// "call," which is to match the highest bet so far made; or
// "raise," which is to increase the previous high bet.

// CHECK MEANS !
// player can check if no bets are set (round is not open)
// If no one has yet opened the betting round, a player may pass or check
// which is equivalent to betting zero
// and/or to calling the current bet of zero.
// In games played with blinds, players may not check on the opening round because the blinds are live bets and must be called or raised to remain in the hand

// NO Checks possible in the pre-flop, because of blinds

// all-in ???? read before make the logic

// To raise is to increase the size of an existing bet
// in the same betting round.

// CALL
// To call is to match a bet or match a raise.

// A betting round ends when all active players have bet an equal amount
// or everyone folds to a player's bet or raise.
// If no opponents call a player's bet or raise, the player wins the pot.

// all-in is a CALL without insufficient stake(chips)
// but may not win any more money from any player above the amount
// of their bet.
// In no-limit games, a player may also go all in, that is,
// betting their entire stack at any point during a betting round.

// If a player does not have sufficient money to cover the ante and
// blinds due, that player is automatically all-in for the coming hand.

//
const AUTO_FOLD_DELAY = 25 // 40 sec

// table states (round)
const WAITING = 'waiting'
const PRE_FLOP = 'preflop'
const FLOP = 'flop'
const TURN = 'turn'
const RIVER = 'river'
const SHOWDOWN = 'showdown'

// actions
const ADD = 'add'
const REMOVE = 'remove'
const BET = 'bet'
const CALL = 'call'
const FOLD = 'fold'
const DEAL = 'deal'
const ALLIN = 'allin'

// player states
const READY = 'ready'
const SITTING = 'sitting'
const CALLED = 'called'
const BETTED = 'betted'
const RAISED = 'raised'
const CHECKED = 'checked'
const FOLDED = 'folded'

const REQUESTED_INVOICE = 'requested'
const PENDING_INVOICE = 'pending'
const SETTLED_INVOICE = 'settled'

const REQUESTED_PAYMENT = 'requested'
const PENDING_PAYMENT = 'pending'
const CONFIRMED_PAYMENT = 'confirmed'
const ERROR_PAYMENT = 'error'

// TODO: save last action time, because of 30 sec player need to move

module.exports = {
  WAITING,
  PRE_FLOP,
  FLOP,
  TURN,
  RIVER,
  SHOWDOWN,

  // actions
  ADD,
  REMOVE,
  BET,
  CALL,
  FOLD,
  ALLIN,
  DEAL,

  // player states
  READY,
  SITTING,
  CALLED,
  BETTED,
  RAISED,
  CHECKED,
  FOLDED,

  AUTO_FOLD_DELAY,

  REQUESTED_INVOICE,
  PENDING_INVOICE,
  SETTLED_INVOICE,

  REQUESTED_PAYMENT,
  PENDING_PAYMENT,
  CONFIRMED_PAYMENT,
  ERROR_PAYMENT
}
