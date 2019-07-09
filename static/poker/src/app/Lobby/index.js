/** @format */

import React from 'react'
import useTables from '../use-tables'
import {Helmet} from 'react-helmet'
import Table from '@material-ui/core/Table'
import TableBody from '@material-ui/core/TableBody'
import TableCell from '@material-ui/core/TableCell'
import TableHead from '@material-ui/core/TableHead'
import TableRow from '@material-ui/core/TableRow'

const Lobby = ({history}) => {
  const {tables = []} = useTables()

  const handleClick = ({id}) => event => {
    history.push(`/${id}`)
  }

  return (
    <div className="lobby">
      <Helmet>{/* <html className="green" /> */}</Helmet>
      <Table className={'poker-tables'}>
        <TableHead>
          <TableRow>
            <TableCell>blinds</TableCell>
            <TableCell align="right">buy-in</TableCell>
            <TableCell align="right">players</TableCell>
            <TableCell align="right">rake %</TableCell>
            <TableCell align="right">pot</TableCell>
            <TableCell>round</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {tables.map((row, key) => (
            <TableRow key={key} hover onClick={handleClick(row)}>
              <TableCell scope="row">
                {row.smallBlind}/{row.bigBlind}
              </TableCell>
              <TableCell align="right">{row.buyIn}</TableCell>
              <TableCell align="right">{row.playing}</TableCell>
              <TableCell align="right">{row.rake}</TableCell>
              <TableCell align="right">{row.pot}</TableCell>
              <TableCell>{row.round}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export default Lobby
