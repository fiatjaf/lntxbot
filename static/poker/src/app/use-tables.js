/** @format */

import firebase from 'firebase/app'
import {useCollection} from 'react-firebase-hooks/firestore'

// TODO: change database rules, add only if tableId provided, do not allow dump all players
export default tableId => {
  const [value, loading, error] = useCollection(
    firebase
      .firestore()
      .collection('tables')
      .limit(20)
  )

  let tables = []
  if (value) {
    value.docs.forEach(doc => {
      tables.push(Object.assign({id: doc.ref.id}, doc.data()))
    })
    tables.sort((a, b) => a.smallBlind - b.smallBlind)
  }

  return {
    loading,
    error,
    tables
  }
}
