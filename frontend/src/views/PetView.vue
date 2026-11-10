<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Events, Window } from '@wailsio/runtime'
import * as Pet from '../../bindings/pcserve/petservice'
import { Config } from '../../bindings/pcserve/models'
import { PetStage, type ScreenRect } from '../pet/stage'
import { playSound, playVoice, stopVoice, type SoundName } from '../pet/sound'
import type { PetKind, PetState } from '../pet/types'

const SLEEP_AFTER = 3 * 60_000

const canvas = ref<HTMLCanvasElement>()
const bubbleEl = ref<HTMLElement>()
const menuEl = ref<HTMLElement>()
const inputEl = ref<HTMLInputElement>()

const bubble = reactive({ shown: false, text: '', kind: '' })
const chat = reactive({ open: false, text: '', thinking: false })
const menu = reactive({ shown: false, x: 0, y: 0 })

let stage: PetStage
let cfg = new Config()
let lastInteraction = Date.now()
let nextDecision = Date.now() + 8000
let revertTimer = 0
let hideTimer = 0
let typeTimer = 0
let walking = false
let lastRegion = ''
const timers: number[] = []
const unsubs: (() => void)[] = []

const drag = { pending: false, sx: 0, sy: 0 }
let clickTimer = 0

const kind = () => cfg.pet as PetKind
const sleeping = () => stage.state === 'sleep'

function sound(name: SoundName) {
  if (!cfg.muted) void playSound(kind(), name)
}

function act(state: PetState, ms?: number) {
  clearTimeout(revertTimer)
  stage.setState(state)
  if (ms) revertTimer = window.setTimeout(() => stage.setState('idle'), ms)
}

function wake() {
  lastInteraction = Date.now()
  nextDecision = Date.now() + 10_000
}

function say(text: string, kind = 'chat') {
  clearTimeout(hideTimer)
  clearInterval(typeTimer)
  bubble.kind = kind
  bubble.text = ''
  bubble.shown = true
  const chars = [...text]
  let i = 0
  typeTimer = window.setInterval(() => {
    bubble.text += chars[i++] ?? ''
    if (i >= chars.length) clearInterval(typeTimer)
  }, 45)

  const voiced = cfg.voice.enabled && !cfg.muted
  if (kind === 'rest') {
    act('remind', 4000)
    sound('remind')
    hideTimer = window.setTimeout(hideBubble, 60_000)
  } else {
    act('talk', Math.min(5000, chars.length * 110 + 900))
    if (!voiced) sound('talk')
    hideTimer = window.setTimeout(hideBubble, chars.length * 150 + 5000)
  }
  if (voiced) void speak(text, kind)
}

let speakToken = 0

async function speak(text: string, kind: string) {
  const token = ++speakToken
  stopVoice()
  try {
    const audio = await Pet.Speak(text)
    if (!audio || token !== speakToken) return
    const seconds = await playVoice(audio)
    if (token !== speakToken) return
    if (kind !== 'rest') act('talk', seconds * 1000 + 300)
    if (kind !== 'rest' && bubble.shown) {
      clearTimeout(hideTimer)
      hideTimer = window.setTimeout(hideBubble, seconds * 1000 + 4000)
    }
  } catch (e) {
    console.warn('语音合成失败', e)
  }
}

function hideBubble() {
  clearInterval(typeTimer)
  bubble.shown = false
}

async function openChat() {
  wake()
  menu.shown = false
  chat.open = true
  if (sleeping()) act('happy', 1200)
  await nextTick()
  inputEl.value?.focus()
}

async function sendChat() {
  const text = chat.text.trim()
  if (!text || chat.thinking) return
  chat.thinking = true
  chat.text = ''
  clearTimeout(hideTimer)
  bubble.kind = 'chat'
  bubble.text = '……'
  bubble.shown = true
  act('sit')
  try {
    say(await Pet.Chat(text))
  } finally {
    chat.thinking = false
    inputEl.value?.focus()
  }
}

function closeChat() {
  chat.open = false
  chat.text = ''
}

async function onPetClick() {
  wake()
  act('happy', 1300)
  sound('happy')
  if (Math.random() < 0.4) say(await Pet.PetLine(), 'pet')
}

function onPointerDown(e: PointerEvent) {
  if (e.button !== 0) return
  menu.shown = false
  if (!stage.hit(e.clientX, e.clientY)) return
  canvas.value!.setPointerCapture(e.pointerId)
  drag.pending = true
  drag.sx = e.screenX
  drag.sy = e.screenY
}

function onPointerMove(e: PointerEvent) {
  stage.setLook(e.clientX)
  if (!drag.pending || Math.hypot(e.screenX - drag.sx, e.screenY - drag.sy) <= 5) return
  drag.pending = false
  canvas.value!.releasePointerCapture(e.pointerId)
  walking = false
  wake()
  stage.face(0)
  act('idle')
  void Pet.StartDrag()
}

function onPointerUp(e: PointerEvent) {
  if (!drag.pending) return
  canvas.value!.releasePointerCapture(e.pointerId)
  drag.pending = false
  if (clickTimer) {
    clearTimeout(clickTimer)
    clickTimer = 0
    void openChat()
  } else {
    clickTimer = window.setTimeout(() => {
      clickTimer = 0
      void onPetClick()
    }, 260)
  }
}

function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  if (!stage.hit(e.clientX, e.clientY)) return
  menu.x = Math.min(e.clientX, window.innerWidth - 130)
  menu.y = Math.min(e.clientY, window.innerHeight - 150)
  menu.shown = true
}

async function menuAction(action: 'chat' | 'settings' | 'mute' | 'hide') {
  menu.shown = false
  if (action === 'chat') void openChat()
  if (action === 'settings') void Pet.OpenSettings()
  if (action === 'mute') void Pet.SetMuted(!cfg.muted)
  if (action === 'hide') void Pet.HidePet()
}

async function restOk() {
  hideBubble()
  wake()
  act('happy', 1300)
  sound('happy')
}

async function restLater() {
  hideBubble()
  await Pet.Snooze(10)
  say('好吧，10 分钟后我再来叫你～', 'chat')
}

async function walk() {
  const [wa, pos] = await Promise.all([Pet.GetWorkArea(), Window.Position()])
  const minX = wa.x - 40
  const maxX = wa.x + wa.width - window.innerWidth + 40
  const dist = (120 + Math.random() * 220) * (Math.random() < 0.5 ? -1 : 1)
  const target = Math.max(minX, Math.min(maxX, pos.x + dist))
  if (Math.abs(target - pos.x) < 60) return
  walking = true
  stage.face(Math.sign(target - pos.x))
  act('walk')
  await Pet.Walk(Math.round(target))
  walking = false
  stage.face(0)
  if (stage.state === 'walk') act('idle')
}

function think() {
  if (walking || drag.pending || chat.open || bubble.kind === 'rest' && bubble.shown) return
  const now = Date.now()
  if (now - lastInteraction > SLEEP_AFTER) {
    if (!sleeping()) act('sleep')
    return
  }
  if (now < nextDecision || stage.state !== 'idle') return
  nextDecision = now + 9000 + Math.random() * 9000
  const r = Math.random()
  if (r < 0.45) void walk()
  else if (r < 0.7) act('sit', 6000 + Math.random() * 4000)
}

function inflate(r: ScreenRect, pad: number): ScreenRect {
  return { x: r.x - pad, y: r.y - pad, w: r.w + pad * 2, h: r.h + pad * 2 }
}

function domRect(el?: HTMLElement | null): ScreenRect | null {
  if (!el) return null
  const r = el.getBoundingClientRect()
  return r.width ? { x: r.left, y: r.top, w: r.width, h: r.height } : null
}

function updateHitRegion() {
  let rects: ScreenRect[]
  if (drag.pending) {
    rects = [{ x: 0, y: 0, w: window.innerWidth, h: window.innerHeight }]
  } else {
    const pet = stage.screenRect(true)
    const bubble = domRect(bubbleEl.value)
    const menuBox = domRect(menuEl.value)
    rects = [pet && inflate(pet, 6), bubble && inflate(bubble, 16), menuBox && inflate(menuBox, 16)].filter(
      (r): r is ScreenRect => !!r,
    )
  }
  const dpr = window.devicePixelRatio
  const physical = rects.map((r) => ({
    x: Math.floor(r.x * dpr),
    y: Math.floor(r.y * dpr),
    w: Math.ceil(r.w * dpr),
    h: Math.ceil(r.h * dpr),
  }))
  const key = JSON.stringify(physical)
  if (key === lastRegion) return
  lastRegion = key
  void Pet.SetHitRegion(physical)
}

async function applyConfig(next: Config) {
  const changedPet = next.pet !== cfg.pet
  cfg = next
  if (cfg.muted || !cfg.voice.enabled) stopVoice()
  if (changedPet) {
    await stage.setPet(kind())
    act('happy', 1300)
    sound('happy')
  }
}

onMounted(async () => {
  stage = new PetStage(canvas.value!)
  const initial = await Pet.GetConfig()
  cfg = initial
  await stage.setPet(kind())

  unsubs.push(
    Events.On('config:changed', (e: any) => void applyConfig(Config.createFrom(e.data))),
    Events.On('pet:say', (e: any) => {
      wake()
      say(e.data.text, e.data.kind)
    }),
    Events.On('pet:chat', () => void openChat()),
  )
  timers.push(window.setInterval(think, 1000), window.setInterval(updateHitRegion, 120))
})

onBeforeUnmount(() => {
  timers.forEach(clearInterval)
  unsubs.forEach((u) => u())
  stage?.dispose()
})
</script>

<template>
  <div class="pet-root" @contextmenu="onContextMenu">
    <canvas
      ref="canvas"
      class="stage"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
    />

    <div v-if="bubble.shown || chat.open" ref="bubbleEl" class="bubble">
      <p v-if="bubble.shown" class="text">{{ bubble.text }}</p>
      <div v-if="bubble.shown && bubble.kind === 'rest'" class="actions">
        <button @click="restOk">好，休息一下</button>
        <button class="ghost" @click="restLater">10 分钟后</button>
      </div>
      <form v-if="chat.open" class="chat" @submit.prevent="sendChat">
        <input
          ref="inputEl"
          v-model="chat.text"
          :disabled="chat.thinking"
          maxlength="200"
          :placeholder="chat.thinking ? '它在想…' : '跟它说点什么'"
          @keydown.esc="closeChat"
        />
        <button type="button" class="ghost close" aria-label="关闭聊天" @click="closeChat">×</button>
      </form>
    </div>

    <ul v-if="menu.shown" ref="menuEl" class="menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }">
      <li @click="menuAction('chat')">和它聊天</li>
      <li @click="menuAction('mute')">{{ cfg.muted ? '取消静音' : '静音' }}</li>
      <li @click="menuAction('settings')">设置…</li>
      <li @click="menuAction('hide')">先藏起来</li>
    </ul>
  </div>
</template>

<style scoped>
.pet-root {
  position: fixed;
  inset: 0;
}

.stage {
  width: 100%;
  height: 100%;
  display: block;
  cursor: grab;
}

.bubble {
  position: absolute;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 260px;
  min-width: 120px;
  padding: 10px 12px;
  border-radius: 14px;
  background: #fffdf8;
  color: #3b3530;
  border: 1.5px solid #f0dcc4;
  box-shadow: 0 4px 14px rgba(80, 50, 20, 0.18);
  font-size: 13px;
  line-height: 1.5;
}

.bubble::after {
  content: "";
  position: absolute;
  bottom: -7px;
  left: 50%;
  width: 12px;
  height: 12px;
  background: #fffdf8;
  border-right: 1.5px solid #f0dcc4;
  border-bottom: 1.5px solid #f0dcc4;
  transform: translateX(-50%) rotate(45deg);
}

.text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.actions {
  display: flex;
  gap: 6px;
  margin-top: 8px;
}

.chat {
  display: flex;
  gap: 4px;
  margin-top: 6px;
}

.text + .chat {
  margin-top: 8px;
}

.chat input {
  flex: 1;
  min-width: 0;
  padding: 5px 8px;
  border: 1px solid #e6d3bd;
  border-radius: 8px;
  font: inherit;
  outline: none;
  background: #fff;
}

.chat input:focus {
  border-color: #f4a259;
}

button {
  padding: 4px 10px;
  border: none;
  border-radius: 8px;
  background: #f4a259;
  color: #fff;
  font: inherit;
  cursor: pointer;
}

button.ghost {
  background: #f6eadb;
  color: #7a5a3a;
}

button.close {
  padding: 0 8px;
  font-size: 16px;
}

.menu {
  position: absolute;
  margin: 0;
  padding: 4px;
  list-style: none;
  min-width: 110px;
  background: #fffdf8;
  border: 1px solid #f0dcc4;
  border-radius: 10px;
  box-shadow: 0 4px 14px rgba(80, 50, 20, 0.18);
  font-size: 13px;
  color: #3b3530;
}

.menu li {
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
}

.menu li:hover {
  background: #f6eadb;
}
</style>
