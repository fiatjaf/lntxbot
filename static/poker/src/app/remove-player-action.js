/** @format */

import React, {useState} from 'react'
import Button from '@material-ui/core/Button'
import LeaveIcon from '@material-ui/icons/ExitToApp'

import useReactRouter from 'use-react-router'
import useAccount from './use-account'

import {removePlayer} from './actions'

export default ({playerId}) => {
  const {match} = useReactRouter()
  const {tableId} = match.params
  const {account: {accountId} = {}} = useAccount()

  const [disabled, setDisabled] = useState(false)

  // add player action
  const handleAction = async event => {
    setDisabled(true)
    const {error} = await removePlayer(tableId, accountId, playerId)
    if (error) {
      setDisabled(false)
    }
  }

  return (
    <Button title="Leave table" disabled={disabled} onClick={handleAction}>
      <LeaveIcon />
    </Button>
  )
}
