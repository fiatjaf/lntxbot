/** @format */

import React from 'react'
import Progress from './progress'
import {READY, SITTING} from '../../lib/types'
import Tooltip from '@material-ui/core/Tooltip'

export default ({
  bet,
  state,
  allin,
  winner,
  foldAt,
  position,
  profit = 0,
  dealer,
  chipsBet = 0
}) => {
  let text = state

  // const [open, setOpen] = React.useState(true);

  if (allin) {
    text = 'all-in'
  }

  const pureProfit = profit - chipsBet

  if (state === READY) {
    text = ' '
  }

  if (winner && pureProfit > 0) {
    text = 'winner'
  }

  if (state === SITTING) {
    text = SITTING
  }

  let placement = 'top'
  if (position > 2 && position < 8) {
    placement = 'bottom'
  }

  return (
    <Tooltip
      open={pureProfit > 0}
      title={`+${pureProfit}`}
      placement={placement}
    >
      <div className="current-bet">
        <div className="bet">{bet === 0 ? '' : bet}</div>
        {foldAt && state !== SITTING ? (
          <Progress foldAt={foldAt} />
        ) : (
          <div className="state">{text}</div>
        )}
      </div>
    </Tooltip>
  )
}
