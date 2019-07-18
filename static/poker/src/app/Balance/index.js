/** @format */

import React from 'react'
import Snackbar from '@material-ui/core/Snackbar'
import Button from '@material-ui/core/Button'
import useReactRouter from 'use-react-router'

import useAccount from '../use-account'
import usePlayers from '../use-players'

const Balance = ({balance}) => {
  const {history, location} = useReactRouter()
  const {account: {hash} = {}} = useAccount()

  let tableId = location.pathname
    .slice(1)
    .split('?')[0]
    .split('/')[0]
  let {players = {}} = usePlayers(tableId)

  var mePlaying = false
  if (tableId !== '') {
    mePlaying = !!Object.values(players).find(p => p.accountHash === hash)
  }

  return (
    <div id="permanent-balance">
      {!mePlaying && (
        <Snackbar
          anchorOrigin={{
            vertical: location.pathname === '/' ? 'bottom' : 'top',
            horizontal: 'right'
          }}
          open={true}
          message={`Balance: ${balance} sats`}
          action={
            location.pathname !== '/' && (
              <Button
                color="inherit"
                size="small"
                onClick={() => {
                  history.push('/')
                }}
              >
                Lobby
              </Button>
            )
          }
        />
      )}
    </div>
  )
}

export default Balance
