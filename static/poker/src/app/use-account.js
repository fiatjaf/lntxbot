/** @format */

import firebase from 'firebase/app'
import {useEffect} from 'react'
import {useDocument} from 'react-firebase-hooks/firestore'
import once from 'lodash.once'

const createAccount = once(async ref => {
  let ip = {
    origin: '-'
  }

  try {
    ip = await window.fetch('https://httpbin.org/ip').then(r => r.json())
  } finally {
    ref.set({
      userAgent: window.navigator.userAgent,
      referrer: window.document.referrer,
      ip: ip.origin
    })
  }
})

export default () => {
  let accountId = window.localStorage.getItem('accountId')
  if (!accountId) {
    accountId = firebase
      .firestore()
      .collection('accounts')
      .doc().id
    window.localStorage.setItem('accountId', accountId)
  }

  const [value, loading] = useDocument(
    firebase.firestore().doc(`accounts/${accountId}`)
  )

  useEffect(() => {
    if (value && !value.exists) {
      createAccount(value.ref)
    }
  })

  return {
    loading,
    account: value && {
      ...value.data(),
      accountId
    }
  }
}
