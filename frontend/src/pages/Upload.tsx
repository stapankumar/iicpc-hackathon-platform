import { useState } from 'react'

export default function Upload() {
  const [file, setFile] = useState<File | null>(null)
  const [status, setStatus] = useState<'idle' | 'uploading' | 'success' | 'error'>('idle')
  const [submissionID, setSubmissionID] = useState('')
  const [error, setError] = useState('')

  const handleSubmit = async () => {
    if (!file) return

    setStatus('uploading')
    const formData = new FormData()
    formData.append('submission', file)

    try {
      const res = await fetch(`${import.meta.env.VITE_SUBMISSION_URL || 'http://iicpc.local'}/submit`, {
        method: 'POST',
        body: formData,
      })
      const data = await res.json()
      if (res.ok) {
        setSubmissionID(data.submission_id)
        setStatus('success')
      } else {
        setError(data.error || 'Upload failed')
        setStatus('error')
      }
    } catch (e) {
      setError('Could not reach submission service')
      setStatus('error')
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-8">
      <h1 className="text-3xl font-bold">Submit Your Orderbook</h1>

      {/* Rules */}
      <div className="bg-gray-900 rounded-xl p-6 border border-gray-800 space-y-3">
        <h2 className="text-lg font-semibold text-green-400">Submission Rules</h2>
        <ul className="space-y-2 text-gray-300 text-sm">
          <li className="flex gap-2">
            <span className="text-green-400">→</span>
            Submit a <code className="bg-gray-800 px-1 rounded">.zip</code> file containing your code and a <code className="bg-gray-800 px-1 rounded">Dockerfile</code>
          </li>
          <li className="flex gap-2">
            <span className="text-green-400">→</span>
            Your server must expose <code className="bg-gray-800 px-1 rounded">POST /order</code>, <code className="bg-gray-800 px-1 rounded">DELETE /order</code>, <code className="bg-gray-800 px-1 rounded">GET /orderbook</code>
          </li>
          <li className="flex gap-2">
            <span className="text-green-400">→</span>
            Server must listen on port <code className="bg-gray-800 px-1 rounded">8080</code>
          </li>
          <li className="flex gap-2">
            <span className="text-green-400">→</span>
            Resource limits: <code className="bg-gray-800 px-1 rounded">2 CPU cores</code>, <code className="bg-gray-800 px-1 rounded">512MB RAM</code>
          </li>
          <li className="flex gap-2">
            <span className="text-green-400">→</span>
            Scoring: 40% TPS + 40% latency (1/p99) + 20% correctness
          </li>
        </ul>
      </div>

      {/* Upload Box */}
      <div className="bg-gray-900 rounded-xl p-6 border border-gray-800 space-y-4">
        <h2 className="text-lg font-semibold">Upload Submission</h2>

        <div
          className="border-2 border-dashed border-gray-700 rounded-lg p-8 text-center cursor-pointer hover:border-green-500 transition-colors"
          onClick={() => document.getElementById('fileInput')?.click()}
        >
          <input
            id="fileInput"
            type="file"
            accept=".zip"
            className="hidden"
            onChange={(e) => setFile(e.target.files?.[0] || null)}
          />
          {file ? (
            <div className="space-y-1">
              <p className="text-green-400 font-medium">{file.name}</p>
              <p className="text-gray-500 text-sm">{(file.size / 1024).toFixed(1)} KB</p>
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-gray-400">Click to select your .zip file</p>
              <p className="text-gray-600 text-sm">Max 50MB</p>
            </div>
          )}
        </div>

        <button
          onClick={handleSubmit}
          disabled={!file || status === 'uploading'}
          className="w-full py-3 bg-green-500 hover:bg-green-400 disabled:bg-gray-700 disabled:text-gray-500 text-black font-bold rounded-lg transition-colors"
        >
          {status === 'uploading' ? 'Submitting...' : 'Submit'}
        </button>

        {status === 'success' && (
          <div className="bg-green-900 border border-green-700 rounded-lg p-4">
            <p className="text-green-300 font-medium">Submission received!</p>
            <p className="text-green-500 text-sm font-mono mt-1">ID: {submissionID}</p>
            <p className="text-gray-400 text-sm mt-1">Check the leaderboard for your results.</p>
          </div>
        )}

        {status === 'error' && (
          <div className="bg-red-900 border border-red-700 rounded-lg p-4">
            <p className="text-red-300">{error}</p>
          </div>
        )}
      </div>
    </div>
  )
}