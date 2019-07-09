/** @format */

import React from 'react'

import Card, {CardBack} from '../Cards'
import useTable from '../use-table'

export default ({tableId, bets}) => {
  const {table: {cards = [], round, pot} = {}} = useTable(tableId)

  return (
    <div className="board">
      <div className="round">{round}</div>
      <div className="empty cards">
        {cards[0] ? <Card {...cards[0]} /> : <CardBack />}
        {cards[1] ? <Card {...cards[1]} /> : <CardBack />}
        {cards[2] ? <Card {...cards[2]} /> : <CardBack />}
        {cards[3] ? <Card {...cards[3]} /> : <CardBack />}
        {cards[4] ? <Card {...cards[4]} /> : <CardBack />}
      </div>
      <div className="pot">{pot}</div>
    </div>
  )
}
