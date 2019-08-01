/** @format */

import React, {useContext} from 'react'
// import Fab from "@material-ui/core/Fab";
import Button from '@material-ui/core/Button'

import AddIcon from '@material-ui/icons/Add'

import useReactRouter from 'use-react-router'
import useAccount from './use-account'

import {addPlayer} from './actions'
import {AppContext} from './App'

export default ({position, buyIn}) => {
  const {match} = useReactRouter()
  const {tableId} = match.params
  const {loading, account: {accountId, balance} = {}} = useAccount()
  const {showError} = useContext(AppContext)

  // add player action
  const handleAction = async event => {
    if (balance < buyIn) {
      // this will tell the server to refill the user's balance
      window.fetch('/app/poker/deposit', {
        method: 'POST',
        body: `satoshis=${buyIn - balance}`,
        headers: {
          'X-Bot-Poker-Token': window.btoa(
            localStorage.getItem('botId') + '~' + accountId
          ),
          'Content-Type': 'application/x-www-form-urlencoded'
        }
      })

      showError('Please wait while your balance is refilled then try again.')
      return
    }

    // this well tell the server we are online so it can show our name to other players
    window.fetch('/app/poker/playing', {
      method: 'POST',
      headers: {
        'X-Bot-Poker-Token': window.btoa(
          localStorage.getItem('botId') + '~' + accountId
        )
      }
    })

    const {error} = await addPlayer(tableId, accountId, position)
    if (error) {
      showError(error)
    }
  }

  return (
    <Button
      onClick={handleAction}
      className="add-player"
      disabled={loading}
      size="small"
      variant="outlined"
      aria-label="Add"
      title="Sit here"
    >
      <AddIcon />
      {/* sit */}
    </Button>
  )
}
