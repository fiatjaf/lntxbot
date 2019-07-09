/** @format */

import React from 'react'
import Card, {CardBack} from '../Cards'
import className from 'classnames'

export default ({cards = [], empty}) => {
  return (
    <div className={className('cards', {empty})}>
      {cards[0] ? <Card {...cards[0]} /> : <CardBack />}
      {cards[1] ? <Card {...cards[1]} /> : <CardBack />}
    </div>
  )
}
