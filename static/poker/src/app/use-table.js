/** @format */

import firebase from 'firebase/app'
import {useDocument} from 'react-firebase-hooks/firestore'

export default tableId => {
  // value is a DocumentSnapshot
  const [value, loading, error] = useDocument(
    firebase.firestore().doc(`tables/${tableId}`)
  )

  return {
    loading,
    error,
    table: value && value.data()
  }
}
