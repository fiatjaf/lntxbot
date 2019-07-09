/** @format */

import React, {useContext} from 'react'
import className from 'classnames'
import Button from '@material-ui/core/Button'
import Drawer from '@material-ui/core/Drawer'
import {withStyles} from '@material-ui/core/styles'
import {bet, call, fold, allin} from '../actions'

import blue from '@material-ui/core/colors/blue'
import orange from '@material-ui/core/colors/orange'
import red from '@material-ui/core/colors/red'
import purple from '@material-ui/core/colors/purple'

import {AppContext} from '../App'

const styles = theme => ({
  allin: {
    color: theme.palette.getContrastText(purple[500]),
    backgroundColor: purple[500],
    '&:hover': {
      backgroundColor: purple[700]
    }
  },
  bet: {
    color: theme.palette.getContrastText(orange[500]),
    backgroundColor: orange[500],
    '&:hover': {
      backgroundColor: orange[700]
    }
  },
  call: {
    color: theme.palette.getContrastText(blue[500]),
    backgroundColor: blue[500],
    '&:hover': {
      backgroundColor: blue[700]
    }
  },
  fold: {
    color: theme.palette.getContrastText(red[500]),
    backgroundColor: red[500],
    '&:hover': {
      backgroundColor: red[700]
    }
  }
})

const Actions = ({
  id,
  active,
  classes,
  bets,
  maxBet,
  tableId,
  chips,
  bigBlind,
  pot
}) => {
  const disabled = !active
  const canCheck = maxBet === 0
  const canBet = maxBet === 0

  const hasNoChips = maxBet > chips

  const [state, setState] = React.useState({
    bottom: false
  })

  const {showError} = useContext(AppContext)

  const toggleDrawer = (side, open) => () => {
    setState({...state, [side]: open})
  }

  const unique = (value, index, self) => self.indexOf(value) === index

  const betAmounts = () => {
    if (canBet) {
      if (pot > 0) {
        return [
          Math.round(pot / 4),
          Math.round(pot / 3),
          Math.round(pot / 2),
          Math.round((2 * pot) / 3),
          Math.round(pot),
          Math.round(1.5 * pot),
          Math.round(2 * pot),
          Math.round(2.5 * pot),
          Math.round(3 * pot)
        ]
          .filter(unique)
          .reverse()
      } else {
        return [
          bigBlind,
          bigBlind * 2,
          bigBlind * 3,
          bigBlind * 4,
          bigBlind * 5,
          bigBlind * 6,
          bigBlind * 7,
          bigBlind * 8
        ].reverse()
      }
    } else {
      return [
        maxBet * 2,
        maxBet * 3,
        maxBet * 4,
        maxBet * 5,
        maxBet * 6,
        maxBet * 7
      ].reverse()
    }
  }

  const caption = amount => (
    <React.Fragment>
      <span className="text">{canBet ? 'Bet' : 'Raise'}</span>
      <span>&nbsp;{amount}</span>
    </React.Fragment>
  )

  return (
    <div className="actions">
      <Button
        className={className(classes.allin)}
        disabled={disabled}
        variant="contained"
        color="primary"
        onClick={async () => {
          const {error} = await allin(tableId, id)
          if (error) {
            showError(error)
          }
        }}
      >
        All-In
      </Button>
      <Button
        className={className(classes.bet)}
        disabled={disabled || hasNoChips}
        variant="contained"
        color="primary"
        onClick={toggleDrawer('bottom', true)}
      >
        {canBet ? 'Bet' : 'Raise'}
      </Button>
      <Button
        className={className(classes.call)}
        disabled={disabled || hasNoChips}
        variant="contained"
        color="secondary"
        onClick={async () => {
          const {error} = await call(tableId, id)
          if (error) {
            showError(error)
          }
        }}
      >
        {canCheck ? 'Check' : 'Call'}
      </Button>
      <Button
        className={className(classes.fold)}
        disabled={disabled}
        variant="contained"
        onClick={async () => {
          const {error} = await fold(tableId, id)
          if (error) {
            showError(error)
          }
        }}
      >
        Fold
      </Button>
      <Drawer
        anchor="bottom"
        open={state.bottom}
        onClose={toggleDrawer('bottom', false)}
      >
        <div
          tabIndex={0}
          id="raise-bet-options"
          role="button"
          onClick={toggleDrawer('bottom', false)}
          onKeyDown={toggleDrawer('bottom', false)}
        >
          <Button
            fullWidth
            className={className(classes.allin)}
            disabled={disabled}
            variant="contained"
            color="primary"
            onClick={async () => {
              const {error} = await allin(tableId, id)
              if (error) {
                showError(error)
              }
            }}
          >
            All-In
          </Button>
          {betAmounts().map((amount, index) => (
            <Button
              key={amount}
              variant="contained"
              fullWidth
              onClick={async () => {
                const {error} = await bet(tableId, id, amount)
                if (error) {
                  showError(error)
                }
              }}
              className={className(classes.bet)}
            >
              {caption(amount)}
            </Button>
          ))}
        </div>
      </Drawer>
    </div>
  )
}

export default withStyles(styles)(Actions)
