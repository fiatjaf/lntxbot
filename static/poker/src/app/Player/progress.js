/** @format */

import React from 'react'
import LinearProgress from '@material-ui/core/LinearProgress'
import {AUTO_FOLD_DELAY} from '../../lib/types'

export default ({foldAt}) => {
  const [completed, setCompleted] = React.useState(100)

  React.useEffect(() => {
    function progress() {
      const d1 = foldAt.toDate().getTime()
      const d2 = Date.now()
      setCompleted(((d1 - d2) * 100) / (AUTO_FOLD_DELAY * 1000))
    }

    const timer = setInterval(progress, 750)
    return () => {
      clearInterval(timer)
    }
  }, [completed, foldAt])

  return (
    <div className="progress-wrap">
      <LinearProgress variant="determinate" value={completed} />
    </div>
  )
}
