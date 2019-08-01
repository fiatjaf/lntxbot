/** @format */

import React from 'react'
import Cards from './cards'
import className from 'classnames'
import BetAndState from './bet-and-state'
import {SHOWDOWN} from '../../lib/types'
import {useTelegramName} from '../use-telegram'

const CryptoJS = require('crypto-js')

const Player = ({
  active,
  bet,
  cards,
  chips,
  me,
  state,
  winner,
  allin,
  foldAt,
  accountId,
  accountHash,
  round,
  position,
  profit,
  dealer,
  chipsBet,
  xyz
}) => {
  let telegramName = useTelegramName(accountHash)

  if (me && accountId && typeof cards === 'string') {
    // decode cards
    let bytes = CryptoJS.AES.decrypt(cards, accountId)
    cards = JSON.parse(bytes.toString(CryptoJS.enc.Utf8))
  }

  return (
    <React.Fragment>
      <BetAndState
        bet={bet}
        state={state}
        winner={winner}
        allin={allin}
        foldAt={foldAt}
        position={position}
        profit={profit}
        dealer={dealer}
        chipsBet={chipsBet}
      />
      <Cards
        cards={
          me || (round === SHOWDOWN && !(typeof cards === 'string'))
            ? cards
            : []
        }
      />
      <div className={className('chips', {dealer: dealer === position})}>
        {chips} {telegramName ? '@' + telegramName : ''}
      </div>
    </React.Fragment>
  )
}

const Empty = () => {
  return (
    <React.Fragment>
      <div className="current-bet">&nbsp;</div>
      <Cards cards={[]} empty />
      <div className="chips">&nbsp;</div>
    </React.Fragment>
  )
}

export default ({
  id,
  position,
  cards,
  active,
  bet,
  chips,
  me,
  state,
  winner,
  allin,
  children,
  foldAt,
  accountId,
  accountHash,
  round,
  profit,
  dealer,
  chipsBet
}) => {
  return (
    <div
      className={className('player-spot', state, `pl${position}`, {
        winner,
        active
      })}
    >
      {id ? (
        <Player
          active={active}
          bet={bet}
          chips={chips}
          cards={cards}
          me={me}
          state={state}
          winner={winner}
          allin={allin}
          foldAt={foldAt}
          accountId={accountId}
          accountHash={accountHash}
          round={round}
          position={position}
          profit={profit}
          dealer={dealer}
          chipsBet={chipsBet}
        />
      ) : (
        <Empty />
      )}
      {!id && children && <div className="add-player-wrap">{children}</div>}
    </div>
  )
}
