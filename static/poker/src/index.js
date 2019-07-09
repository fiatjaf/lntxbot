/** @format */

import React from 'react'
import {render} from 'react-dom'
import App from './app/App'

import firebase from 'firebase/app'
import 'firebase/firestore'

var config = {
  apiKey: 'AIzaSyCPOIOxFxXUyYcwNwmVZyT-Qb-gQz2w7XQ',
  authDomain: 'ln-pkr.firebaseapp.com',
  databaseURL: 'https://ln-pkr.firebaseio.com',
  projectId: 'ln-pkr'
}

firebase.initializeApp(config)

render(<App />, document.getElementsByTagName('main')[0])
