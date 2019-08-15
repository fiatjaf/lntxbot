/** @format */

import {useState} from 'react'

export function useTelegramName(accountHash) {
  let [name, setName] = useState('')
  if (!accountHash) return ''

  window
    .fetch('/app/poker/online')
    .then(r => r.json())
    .then(map => {
      setName(map[accountHash])
    })

  return name
}
