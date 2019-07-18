/** @format */

import React, {useState} from 'react'
import CssBaseline from '@material-ui/core/CssBaseline'
import {BrowserRouter as Router, Route, Switch} from 'react-router-dom'
import Table from '../Table'
import Lobby from '../Lobby'
import Balance from '../Balance'
import Snackbar from '@material-ui/core/Snackbar'
import Button from '@material-ui/core/Button'

import useAccount from '../use-account'

export const AppContext = React.createContext({})

const App = () => {
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
        <Balance balance={balance} />
        <Route path="/" exact component={Lobby} />
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
