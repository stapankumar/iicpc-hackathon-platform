import { useState, useEffect, useRef } from 'react'

type UploadStatus = 'idle' | 'uploading' | 'building' | 'running' | 'scored' | 'failed' | 'error'

const PIPELINE_STAGES: { key: UploadStatus; label: string }[] = [
  { key: 'uploading', label: 'Uploading' },
  { key: 'building',  label: 'Building image' },
  { key: 'running',   label: 'Judging' },
  { key: 'scored',    label: 'Scored' },
]

const STATUS_MAP: Record<string, UploadStatus> = {
  RECEIVED: 'building',
  BUILDING: 'building',
  RUNNING:  'running',
  SCORED:   'scored',
  FAILED:   'failed',
}

export default function Upload() {
  const [file, setFile]               = useState<File | null>(null)
  const [status, setStatus]           = useState<UploadStatus>('idle')
  const [submissionID, setSubmissionID] = useState('')
  const [error, setError]             = useState('')
  const [teamName, setTeamName] = useState('')
  const pollRef                       = useRef<ReturnType<typeof setInterval> | null>(null)

  // Poll /status until SCORED or FAILED
  useEffect(() => {
    if (!submissionID || status === 'scored' || status === 'failed' || status === 'error') {
      if (pollRef.current) clearInterval(pollRef.current)
      return
    }

    pollRef.current = setInterval(async () => {
      try {
        const res = await fetch(
          `${import.meta.env.VITE_SUBMISSION_URL || 'http://iicpc.local'}/status?submission_id=${submissionID}`
        )
        if (!res.ok) return
        const data = await res.json()
        const mapped = STATUS_MAP[data.status as string]
        if (mapped) setStatus(mapped)
      } catch {}
    }, 3000)

    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [submissionID, status])

  const handleSubmit = async () => {
    if (!file || status !== 'idle') return

    setStatus('uploading')
    setError('')

    const formData = new FormData()
    formData.append('submission', file)
    formData.append('team_name', teamName)

    try {
      const res = await fetch(
        `${import.meta.env.VITE_SUBMISSION_URL || 'http://iicpc.local'}/submit`,
        { method: 'POST', body: formData }
      )
      const data = await res.json()
      if (res.ok) {
        setSubmissionID(data.submission_id)
        setStatus('building')
      } else {
        setError(data.error || 'Upload failed')
        setStatus('error')
      }
    } catch {
      setError('Could not reach submission service')
      setStatus('error')
    }
  }

  const isInProgress = ['uploading', 'building', 'running'].includes(status)
  const isDone = status === 'scored' || status === 'failed'
  const currentStage = status === 'scored'
  ? PIPELINE_STAGES.length  // all stages past
  : PIPELINE_STAGES.findIndex(s => s.key === status)

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
        <input
          type="text"
          placeholder="Team name / your name"
          value={teamName}
          onChange={(e) => setTeamName(e.target.value)}
          disabled={isInProgress || isDone}
          className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-green-500"
        />
        <div
          className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors
            ${isDone || isInProgress
              ? 'border-gray-700 cursor-not-allowed opacity-50'
              : 'border-gray-700 cursor-pointer hover:border-green-500'
            }`}
          onClick={() => {
            if (!isInProgress && !isDone)
              document.getElementById('fileInput')?.click()
          }}
        >
          <input
            id="fileInput"
            type="file"
            accept=".zip"
            className="hidden"
            disabled={isInProgress || isDone}
            onChange={(e) => {
              setFile(e.target.files?.[0] || null)
              if (status === 'scored') {
                setStatus('idle')
                setSubmissionID('')
              }
            }}
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
          disabled={!file || !teamName.trim() || isInProgress || isDone}
          className="w-full py-3 bg-green-500 hover:bg-green-400 disabled:bg-gray-700
                     disabled:text-gray-500 text-black font-bold rounded-lg transition-colors"
        >
          {status === 'uploading' ? 'Uploading...' :
            status === 'building'  ? 'Building...'  :
            status === 'running'   ? 'Judging...'   : 'Submit'}
        </button>

        {/* Pipeline progress — shown after successful upload */}
        {submissionID && (
          <div className="space-y-3">
            <p className="text-xs text-gray-500 font-mono">ID: {submissionID}</p>

            {/* Stage tracker */}
            <div className="flex items-center gap-1">
              {PIPELINE_STAGES.map((stage, i) => {
                const isPast    = i < currentStage
                const isCurrent = i === currentStage
                const isFuture  = i > currentStage

                return (
                  <div key={stage.key} className="flex items-center gap-1 flex-1">
                    <div className="flex flex-col items-center flex-1">
                      <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs
                        ${isPast    ? 'bg-green-500 text-black' : ''}
                        ${isCurrent && status !== 'failed' ? 'bg-yellow-500 text-black animate-pulse' : ''}
                        ${isCurrent && status === 'failed' ? 'bg-red-500 text-white' : ''}
                        ${isFuture  ? 'bg-gray-700 text-gray-500' : ''}
                      `}>
                        {isPast ? '✓' : isCurrent && status === 'failed' ? '✗' : i + 1}
                      </div>
                      <span className={`text-xs mt-1 text-center
                        ${isPast    ? 'text-green-400' : ''}
                        ${isCurrent && status !== 'failed' ? 'text-yellow-400' : ''}
                        ${isCurrent && status === 'failed' ? 'text-red-400' : ''}
                        ${isFuture  ? 'text-gray-600' : ''}
                      `}>
                        {stage.label}
                      </span>
                    </div>
                    {i < PIPELINE_STAGES.length - 1 && (
                      <div className={`h-px flex-1 mb-4 ${isPast ? 'bg-green-500' : 'bg-gray-700'}`} />
                    )}
                  </div>
                )
              })}
            </div>

            {status === 'scored' && (
              <p className="text-green-400 text-sm text-center">
                ✓ Judging complete — check the leaderboard for your rank
              </p>
            )}
            {status === 'failed' && (
              <p className="text-red-400 text-sm text-center">
                Pipeline failed. Check your Dockerfile and try again.
              </p>
            )}
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