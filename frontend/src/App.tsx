import { useState } from 'react'
import Leaderboard from './pages/Leaderboard'
import Upload from './pages/Upload'

export default function App() {
  const [page, setPage] = useState<'leaderboard' | 'upload'>('leaderboard')

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      {/* Navbar */}
      <nav className="bg-gray-900 border-b border-gray-800 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-2xl font-bold text-green-400">IICPC</span>
          <span className="text-gray-400 text-sm">Distributed Benchmarking Platform</span>
        </div>
        <div className="flex gap-4">
          <button
            onClick={() => setPage('leaderboard')}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              page === 'leaderboard'
                ? 'bg-green-500 text-black'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Leaderboard
          </button>
          <button
            onClick={() => setPage('upload')}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              page === 'upload'
                ? 'bg-green-500 text-black'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Submit Code
          </button>
        </div>
      </nav>

      {/* Page Content */}
      <main className="max-w-6xl mx-auto px-6 py-8">
        {page === 'leaderboard' ? <Leaderboard /> : <Upload />}
      </main>
    </div>
  )
}