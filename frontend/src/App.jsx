
import { useState, useEffect } from 'react'

function App() {
  const [apps, setApps] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchApps()
    const interval = setInterval(fetchApps, 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchApps = () => {
    fetch('/api/apps')
      .then(res => res.json())
      .then(data => {
        setApps(data || [])
        setLoading(false)
      })
      .catch(err => {
        console.error(err)
        setLoading(false)
      })
  }

  if (loading) {
    return (
      <div className="loading">
        <div className="spinner"></div>
      </div>
    )
  }

  return (
    <div className="container">
      <header>
        <h1>Redval Server</h1>
        <p className="subtitle">Quick access to your applications</p>
      </header>
      <div className="grid">
        {apps.map((app, i) => (
          <a key={i} href={app.url} className="card" target="_blank" rel="noopener noreferrer">
            <div className="card-icon">
              <img src={app.icon} alt={app.title} />
            </div>
            <div className="card-content">
              <h3>{app.title}</h3>
              <p className="description">{app.description || 'This is a good application'}</p>
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}

export default App