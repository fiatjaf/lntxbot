/** @format */

import React, {useState} from 'react'
import CssBaseline from '@material-ui/core/CssBaseline'
import Header from '../Header'
import {BrowserRouter as Router, Route, Switch} from 'react-router-dom'
import Table from '../Table'
import Lobby from '../Lobby'
import Snackbar from '@material-ui/core/Snackbar'
import Button from '@material-ui/core/Button'
import useReactRouter from 'use-react-router'

import useAccount from '../use-account'

export const AppContext = React.createContext({})

const App = () => {
  const {history, location} = useReactRouter()
  const [errorOpen, setErrorOpen] = useState(false)
  const [errorMessage, setErrorMessage] = useState({})
  const {account: {balance} = {}} = useAccount()

  return (
    <Router>
      <CssBaseline />
      <AppContext.Provider
        value={{
          showError: (text, action) => {
            setErrorMessage({
              text,
              action
            })
            setErrorOpen(true)
          }
        }}
      >
        <Switch>
          <Route path="/:tableId" component={Table} />
        </Switch>
        <Route path="/" exact component={Lobby} />
        <div id="permanent-balance">
          <Snackbar
            anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
            open={true}
            message={`Balance: ${balance} sats`}
            actions={
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
        <Snackbar
          anchorOrigin={{vertical: 'top', horizontal: 'center'}}
          key={'error-message'}
          open={errorOpen}
          onClose={() => {
            setErrorOpen(false)
          }}
          ContentProps={{
            'aria-describedby': 'message-id'
          }}
          message={<span id="message-id">{errorMessage.text}</span>}
          action={
            errorMessage.action && (
              <Button
                color="inherit"
                size="small"
                onClick={() => {
                  errorMessage.action.handle()
                  setErrorOpen(false)
                }}
              >
                {errorMessage.action.title}
              </Button>
            )
          }
        />
      </AppContext.Provider>
    </Router>
  )
}

export default App
