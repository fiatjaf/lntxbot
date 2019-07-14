/** @format */

import React from 'react'
import Snackbar from '@material-ui/core/Snackbar'
import Button from '@material-ui/core/Button'
import useReactRouter from 'use-react-router'

const Balance = ({balance}) => {
  const {history, location} = useReactRouter()

  return (
    <div id="permanent-balance">
      <Snackbar
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
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
    </div>
  )
}

export default Balance
