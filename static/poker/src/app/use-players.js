/** @format */

import firebase from 'firebase/app'
import {useCollection} from 'react-firebase-hooks/firestore'

// TODO: change database rules, add only if tableId provided, do not allow dump all players
export default tableId => {
  const [value, loading, error] = useCollection(
    firebase
      .firestore()
      .collection('players')
      .where('tableId', '==', tableId)
  )

  let players = {}
  if (value) {
    value.docs.forEach(doc => {
      players[doc.get('position') || 0] = Object.assign({}, doc.data(), {
        id: doc.ref.id
      })
    })
  }

  return {
    loading,
    error,
    players
  }
}
