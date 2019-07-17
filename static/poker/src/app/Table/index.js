/** @format */

import React from 'react'
import usePlayers from '../use-players'
import Player from '../Player'
import className from 'classnames'
import useAccount from '../use-account'
import useTable from '../use-table'
import Actions from './actions'
import Board from '../Board'
import Header from '../Header'
import RemovePlayerAction from '../remove-player-action'
import AddPlayerAction from '../add-player-action'

import {Helmet} from 'react-helmet'

const Table = ({match, history}) => {
  const {tableId} = match.params
  const {loading, players = {}} = usePlayers(tableId)
  const {account: {hash, accountId} = {}} = useAccount()
  const me = Object.values(players).find(p => p.accountHash === hash)
  const {
    table: {
      bigBlind = 0,
      round,
      winners = [],
      dealer,
      smallBlind = 0,
      buyIn = 0,
      pot = 0
    } = {}
  } = useTable(tableId)

  const totalBets = Object.values(players).reduce(
    (acc, p) => acc + p.bet || 0,
    0
  )
  const maxBet = Math.max(0, ...Object.values(players).map(({bet}) => bet || 0))

  const renderPlayer = position => {
    const winner = winners.find(w => w.position === position)
    const profit = winner ? winner.amount : 0
    return (
      <Player
        key={position}
        {...players[position]}
        position={position}
        me={position === (me && me.position)}
        accountId={accountId}
        round={round}
        profit={profit}
        dealer={dealer}
      >
        {!loading &&
          !me && <AddPlayerAction position={position} buyIn={buyIn} />}
      </Player>
    )
  }

  return (
    <>
      <Helmet>
        <html className="green" />
      </Helmet>
      <div className={className('table')}>
        <Header>
          <div className="info">{`${smallBlind}/${bigBlind}/${buyIn}`}</div>
          <div className="leave-table">
            {me && <RemovePlayerAction playerId={me.id} />}
          </div>
        </Header>
        {[...Array(10).keys()].map(n => renderPlayer(n + 1))}
        <Board tableId={tableId} bets={totalBets} maxBet={maxBet} />
        {me && (
          <Actions
            {...me}
            bets={totalBets}
            maxBet={maxBet}
            tableId={tableId}
            accountId={accountId}
            chips={me.chips}
            bigBlind={bigBlind}
            pot={pot}
          />
        )}
      </div>
    </>
  )
}

export default Table
