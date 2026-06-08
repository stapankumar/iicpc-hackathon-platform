import { useEffect, useState } from 'react'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'

interface Score {
  submission_id: string
  p50_ms: number
  p90_ms: number
  p99_ms: number
  tps: number
  correctness: number
  score: number
}

export default function Leaderboard() {
  const [scores, setScores]       = useState<Score[]>([])
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const es = new EventSource(
      `${import.meta.env.VITE_LEADERBOARD_URL || 'http://iicpc.local'}/ws`
    )

    es.onopen    = () => setConnected(true)
    es.onerror   = () => setConnected(false)
    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (Array.isArray(data)) setScores(data)
      } catch {}
    }

    return () => es.close()
  }, [])

  return (
    <div className="space-y-8">

      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Live Leaderboard</h1>
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connected ? 'bg-green-400 animate-pulse' : 'bg-red-400'}`} />
          <span className="text-sm text-gray-400">{connected ? 'Live' : 'Disconnected'}</span>
        </div>
      </div>

      {/* Latency Chart */}
      {scores.length > 0 && (
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4 text-gray-300">Latency Distribution (ms)</h2>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={scores}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis
                dataKey="submission_id"
                tick={false}
                axisLine={{ stroke: '#374151' }}
              />
              <YAxis tick={{ fill: '#9CA3AF' }} />
              <Tooltip
                contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151' }}
                labelFormatter={(v) => `ID: ${v}`}
              />
              <Line type="monotone" dataKey="p50_ms" stroke="#34D399" name="p50" strokeWidth={2} dot={false} />
              <Line type="monotone" dataKey="p90_ms" stroke="#FBBF24" name="p90" strokeWidth={2} dot={false} />
              <Line type="monotone" dataKey="p99_ms" stroke="#F87171" name="p99" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Rankings Table */}
      <div className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-800 text-gray-400 uppercase text-xs">
            <tr>
              <th className="px-6 py-3 text-left">Rank</th>
              <th className="px-6 py-3 text-left">Submission ID</th>
              <th className="px-6 py-3 text-right">p50 (ms)</th>
              <th className="px-6 py-3 text-right">p90 (ms)</th>
              <th className="px-6 py-3 text-right">p99 (ms)</th>
              <th className="px-6 py-3 text-right">TPS</th>
              <th className="px-6 py-3 text-right">Correctness</th>
              <th className="px-6 py-3 text-right">Score</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {scores.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-6 py-12 text-center text-gray-500">
                  No submissions yet. Be the first to submit!
                </td>
              </tr>
            ) : (
              scores.map((s, i) => (
                <tr key={s.submission_id} className="hover:bg-gray-800 transition-colors">

                  {/* Rank */}
                  <td className="px-6 py-4">
                    <span className={`font-bold text-lg ${
                      i === 0 ? 'text-yellow-400' :
                      i === 1 ? 'text-gray-300'   :
                      i === 2 ? 'text-amber-600'  : 'text-gray-500'
                    }`}>#{i + 1}</span>
                  </td>

                  {/* Submission ID */}
                  <td className="px-6 py-4 font-mono text-green-400">
                    {s.submission_id.slice(0, 12)}...
                  </td>

                  {/* Latencies */}
                  <td className="px-6 py-4 text-right text-green-300">{s.p50_ms.toFixed(2)}</td>
                  <td className="px-6 py-4 text-right text-yellow-300">{s.p90_ms.toFixed(2)}</td>
                  <td className="px-6 py-4 text-right text-red-300">{s.p99_ms.toFixed(2)}</td>

                  {/* TPS */}
                  <td className="px-6 py-4 text-right text-blue-300">{s.tps.toLocaleString()}</td>

                  {/* Correctness — color coded */}
                  <td className="px-6 py-4 text-right">
                    <span className={`font-mono text-sm ${
                      s.correctness >= 0.8 ? 'text-green-400'  :
                      s.correctness >= 0.5 ? 'text-yellow-400' : 'text-red-400'
                    }`}>
                      {(s.correctness * 100).toFixed(0)}%
                    </span>
                  </td>

                  {/* Score */}
                  <td className="px-6 py-4 text-right">
                    <span className="bg-green-900 text-green-300 px-3 py-1 rounded-full font-bold">
                      {s.score.toFixed(1)}
                    </span>
                  </td>

                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}