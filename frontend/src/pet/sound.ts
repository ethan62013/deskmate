import type { PetKind } from './types'

export type SoundName = 'happy' | 'talk' | 'remind'

let ctx: AudioContext | null = null
const files = new Map<string, Promise<AudioBuffer | null>>()

function audio() {
  ctx ??= new AudioContext()
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

/** Custom sounds can be dropped in public/sounds/<kind>/<name>.mp3 to replace the synthesized ones. */
function loadFile(kind: PetKind, name: SoundName) {
  const key = `${kind}/${name}`
  let p = files.get(key)
  if (!p) {
    p = fetch(`/sounds/${key}.mp3`)
      .then(async (r) => {
        const type = r.headers.get('content-type') ?? ''
        if (!r.ok || type.includes('text/html')) return null
        return audio().decodeAudioData(await r.arrayBuffer())
      })
      .catch(() => null)
    files.set(key, p)
  }
  return p
}

function envelope(ac: AudioContext, at: number, peak: number, attack: number, length: number) {
  const g = ac.createGain()
  g.gain.setValueAtTime(0.0001, at)
  g.gain.exponentialRampToValueAtTime(peak, at + attack)
  g.gain.exponentialRampToValueAtTime(0.0001, at + length)
  g.connect(ac.destination)
  return g
}

function meow(ac: AudioContext, at: number, pitch = 1) {
  const osc = ac.createOscillator()
  osc.type = 'sawtooth'
  osc.frequency.setValueAtTime(430 * pitch, at)
  osc.frequency.linearRampToValueAtTime(680 * pitch, at + 0.16)
  osc.frequency.linearRampToValueAtTime(380 * pitch, at + 0.5)
  const filter = ac.createBiquadFilter()
  filter.type = 'bandpass'
  filter.Q.value = 3
  filter.frequency.setValueAtTime(900, at)
  filter.frequency.linearRampToValueAtTime(1700, at + 0.18)
  filter.frequency.linearRampToValueAtTime(800, at + 0.5)
  osc.connect(filter).connect(envelope(ac, at, 0.35, 0.05, 0.55))
  osc.start(at)
  osc.stop(at + 0.6)
}

function bark(ac: AudioContext, at: number, pitch = 1) {
  const osc = ac.createOscillator()
  osc.type = 'square'
  osc.frequency.setValueAtTime(320 * pitch, at)
  osc.frequency.exponentialRampToValueAtTime(140 * pitch, at + 0.14)
  const filter = ac.createBiquadFilter()
  filter.type = 'lowpass'
  filter.frequency.value = 1400
  osc.connect(filter).connect(envelope(ac, at, 0.25, 0.01, 0.17))
  osc.start(at)
  osc.stop(at + 0.2)

  const noise = ac.createBufferSource()
  const buf = ac.createBuffer(1, ac.sampleRate * 0.12, ac.sampleRate)
  const data = buf.getChannelData(0)
  for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1
  noise.buffer = buf
  const band = ac.createBiquadFilter()
  band.type = 'bandpass'
  band.frequency.value = 900
  noise.connect(band).connect(envelope(ac, at, 0.12, 0.005, 0.12))
  noise.start(at)
}

function chirp(ac: AudioContext, at: number, pitch = 1) {
  const osc = ac.createOscillator()
  osc.type = 'sine'
  osc.frequency.setValueAtTime(1900 * pitch, at)
  osc.frequency.exponentialRampToValueAtTime(3300 * pitch, at + 0.07)
  osc.connect(envelope(ac, at, 0.2, 0.01, 0.09))
  osc.start(at)
  osc.stop(at + 0.1)
}

const voices: Record<PetKind, (ac: AudioContext, at: number, pitch?: number) => void> = {
  cat: meow,
  dog: bark,
  parrot: chirp,
}

const patterns: Record<PetKind, Record<SoundName, [number, number][]>> = {
  cat: { happy: [[0, 1.1]], talk: [[0, 1.25]], remind: [[0, 1], [0.6, 1.15]] },
  dog: { happy: [[0, 1], [0.22, 1.05]], talk: [[0, 1.2]], remind: [[0, 1], [0.2, 1], [0.45, 1.1]] },
  parrot: { happy: [[0, 1], [0.1, 1.1], [0.2, 1.2]], talk: [[0, 1], [0.12, 0.9]], remind: [[0, 1], [0.1, 1], [0.35, 1.2], [0.45, 1.2]] },
}

let voice: AudioBufferSourceNode | null = null

export function stopVoice() {
  voice?.stop()
  voice = null
}

/** Plays base64 mp3 speech, replacing any line still being spoken. Resolves to its duration in seconds. */
export async function playVoice(base64: string) {
  const ac = audio()
  const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
  const buffer = await ac.decodeAudioData(bytes.buffer)
  stopVoice()
  const src = ac.createBufferSource()
  src.buffer = buffer
  src.connect(ac.destination)
  src.onended = () => {
    if (voice === src) voice = null
  }
  src.start()
  voice = src
  return buffer.duration
}

export async function playSound(kind: PetKind, name: SoundName) {
  const ac = audio()
  const buffer = await loadFile(kind, name)
  if (buffer) {
    const src = ac.createBufferSource()
    src.buffer = buffer
    src.connect(ac.destination)
    src.start()
    return
  }
  const now = ac.currentTime + 0.01
  for (const [offset, pitch] of patterns[kind][name]) voices[kind](ac, now + offset, pitch)
}
