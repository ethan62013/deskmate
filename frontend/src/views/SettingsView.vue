<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import * as Pet from '../../bindings/pcserve/petservice'
import { Config } from '../../bindings/pcserve/models'
import { playVoice } from '../pet/sound'

const presets = [
  { label: 'DeepSeek', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
  { label: '通义千问', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus' },
  { label: '智谱', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', model: 'glm-4-flash' },
  { label: '本机 Ollama', baseUrl: 'http://localhost:11434/v1', model: 'qwen2.5' },
]

const pets = [
  { id: 'cat', label: '猫' },
  { id: 'dog', label: '狗' },
  { id: 'parrot', label: '鹦鹉' },
]

const form = reactive(new Config())
const excluded = ref('')
const status = ref<{ text: string; error?: boolean } | null>(null)
const testing = ref(false)
const saving = ref(false)
const previewing = ref(false)
const workMinutes = ref(0)
let timer = 0
let unsub = () => {}

const ready = computed(() => form.pet !== '')

function load(c: Config) {
  Object.assign(form, Config.createFrom(JSON.parse(JSON.stringify(c))))
  excluded.value = form.activity.excludedApps.join('\n')
}

function collect(): Config {
  const c = Config.createFrom(JSON.parse(JSON.stringify(form)))
  c.activity.excludedApps = excluded.value
    .split(/\r?\n/)
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean)
  return c
}

async function save() {
  saving.value = true
  try {
    load(await Pet.SaveConfig(collect()))
    status.value = { text: '已保存' }
  } catch (e: any) {
    status.value = { text: String(e?.message ?? e), error: true }
  } finally {
    saving.value = false
  }
}

async function test() {
  testing.value = true
  status.value = { text: '正在连接模型…' }
  try {
    status.value = { text: `它说：${await Pet.TestLLM(collect())}` }
  } catch (e: any) {
    status.value = { text: `连接失败：${e?.message ?? e}`, error: true }
  } finally {
    testing.value = false
  }
}

async function previewVoice() {
  previewing.value = true
  status.value = { text: '正在合成语音…' }
  try {
    await playVoice(await Pet.TestVoice(collect()))
    status.value = { text: '试听中，满意的话记得保存' }
  } catch (e: any) {
    status.value = { text: `语音合成失败：${e?.message ?? e}`, error: true }
  } finally {
    previewing.value = false
  }
}

function usePreset(p: (typeof presets)[number]) {
  form.llm.baseUrl = p.baseUrl
  form.llm.model = p.model
}

async function refreshWork() {
  workMinutes.value = await Pet.WorkMinutes()
}

onMounted(async () => {
  load(await Pet.GetConfig())
  unsub = Events.On('config:changed', (e: any) => {
    const next = Config.createFrom(e.data)
    form.pet = next.pet
    form.muted = next.muted
  })
  void refreshWork()
  timer = window.setInterval(refreshWork, 30_000)
})

onBeforeUnmount(() => {
  unsub()
  clearInterval(timer)
})
</script>

<template>
  <main v-if="ready" class="settings">
    <h1>宠物设置</h1>

    <section>
      <h2>宠物</h2>
      <div class="pets" role="radiogroup" aria-label="选择宠物">
        <label v-for="p in pets" :key="p.id" :class="{ active: form.pet === p.id }">
          <input v-model="form.pet" type="radio" name="pet" :value="p.id" />
          {{ p.label }}
        </label>
      </div>
      <label class="field">
        <span>名字</span>
        <input v-model="form.name" maxlength="12" />
      </label>
      <label class="check"><input v-model="form.muted" type="checkbox" /> 静音</label>
      <label class="check"><input v-model="form.autoStart" type="checkbox" /> 开机自动启动</label>
    </section>

    <section>
      <h2>对话模型</h2>
      <p class="hint">填写任意 OpenAI 兼容接口。Key 只保存在本机。</p>
      <div class="presets">
        <button v-for="p in presets" :key="p.label" type="button" class="ghost" @click="usePreset(p)">
          {{ p.label }}
        </button>
      </div>
      <label class="field">
        <span>接口地址</span>
        <input v-model="form.llm.baseUrl" placeholder="https://api.deepseek.com/v1" />
      </label>
      <label class="field">
        <span>API Key</span>
        <input v-model="form.llm.apiKey" type="password" autocomplete="off" placeholder="本机 Ollama 可留空" />
      </label>
      <label class="field">
        <span>模型名</span>
        <input v-model="form.llm.model" placeholder="deepseek-chat" />
      </label>
      <button type="button" class="ghost" :disabled="testing" @click="test">测试连接</button>
    </section>

    <section>
      <h2>情侣模式</h2>
      <p class="hint">打开后它会用恋人的口吻和你说话，下面的人设可以随意改。</p>
      <label class="check"><input v-model="form.lover.enabled" type="checkbox" /> 开启情侣模式</label>
      <label class="field">
        <span>它怎么称呼你</span>
        <input v-model="form.lover.callMe" maxlength="12" placeholder="宝贝" />
      </label>
      <label class="field">
        <span>人设</span>
        <textarea v-model="form.lover.persona" class="prose" rows="4" maxlength="500" />
      </label>
    </section>

    <section>
      <h2>语音</h2>
      <p class="hint">
        使用火山引擎豆包语音合成。在控制台开通语音合成后，把 API Key 和音色 ID 填在这里，只保存在本机。
      </p>
      <label class="check"><input v-model="form.voice.enabled" type="checkbox" /> 让它开口说话</label>
      <label class="field">
        <span>API Key（旧版控制台填 Access Token）</span>
        <input v-model="form.voice.apiKey" type="password" autocomplete="off" />
      </label>
      <label class="field">
        <span>App ID（仅旧版控制台需要，新版留空）</span>
        <input v-model="form.voice.appId" autocomplete="off" />
      </label>
      <label class="field">
        <span>模型版本（要和音色所属版本一致）</span>
        <select v-model="form.voice.resourceId">
          <option value="seed-tts-2.0">语音合成 2.0（seed-tts-2.0）</option>
          <option value="seed-tts-1.0">语音合成 1.0（seed-tts-1.0）</option>
        </select>
      </label>
      <label class="field">
        <span>音色 ID（控制台 › 音色库）</span>
        <input v-model="form.voice.speaker" spellcheck="false" />
      </label>
      <label class="field">
        <span>语气指令（仅 2.0 有效，可留空）</span>
        <input v-model="form.voice.style" maxlength="40" placeholder="用温柔亲昵的语气说" />
      </label>
      <label class="field inline">
        <span>语速</span>
        <input v-model.number="form.voice.speechRate" type="number" min="-50" max="100" step="10" />
        <span>（-50 慢 ~ 100 快，0 为正常）</span>
      </label>
      <button type="button" class="ghost" :disabled="previewing" @click="previewVoice">试听</button>
    </section>

    <section>
      <h2>陪伴与休息</h2>
      <p class="hint">已连续工作 {{ workMinutes }} 分钟。只读取软件类别，不读取窗口标题。</p>
      <label class="check">
        <input v-model="form.activity.remarks" type="checkbox" /> 根据我在用的软件说话
      </label>
      <label class="field inline">
        <span>最多每</span>
        <input v-model.number="form.activity.remarkMinutes" type="number" min="5" max="240" />
        <span>分钟说一次</span>
      </label>
      <label class="check">
        <input v-model="form.activity.restReminder" type="checkbox" /> 工作太久提醒休息
      </label>
      <label class="field inline">
        <span>连续工作</span>
        <input v-model.number="form.activity.workMinutes" type="number" min="10" max="240" />
        <span>分钟后提醒</span>
      </label>
      <label class="field">
        <span>不读取这些软件（每行一个进程名）</span>
        <textarea v-model="excluded" rows="4" spellcheck="false" />
      </label>
    </section>

    <footer>
      <p v-if="status" class="status" :class="{ error: status.error }" role="status">{{ status.text }}</p>
      <button type="button" :disabled="saving" @click="save">保存</button>
    </footer>
  </main>
</template>

<style scoped>
.settings {
  box-sizing: border-box;
  max-width: 520px;
  margin: 0 auto;
  padding: 20px 24px 88px;
  color: #3b3530;
  background: #faf8f5;
  min-height: 100%;
  font-size: 14px;
}

h1 {
  margin: 0 0 16px;
  font-size: 20px;
  font-weight: 500;
}

h2 {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 500;
}

section {
  padding: 16px;
  margin-bottom: 14px;
  background: #fff;
  border: 1px solid #efe4d6;
  border-radius: 12px;
}

.hint {
  margin: -4px 0 10px;
  color: #8a7c6e;
  font-size: 12px;
}

.pets {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.pets label {
  flex: 1;
  padding: 8px 0;
  text-align: center;
  border: 1px solid #ead9c4;
  border-radius: 10px;
  cursor: pointer;
}

.pets label.active {
  border-color: #f4a259;
  background: #fff3e6;
  color: #b0601e;
}

.pets input {
  position: absolute;
  opacity: 0;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 10px;
}

.field span {
  color: #6f6255;
  font-size: 13px;
}

.field.inline {
  flex-direction: row;
  align-items: center;
  gap: 8px;
  padding-left: 24px;
}

.field.inline input {
  width: 64px;
}

input:not([type="checkbox"]):not([type="radio"]),
select,
textarea {
  padding: 7px 9px;
  border: 1px solid #e6d3bd;
  border-radius: 8px;
  font: inherit;
  background: #fff;
  outline: none;
}

input:focus,
textarea:focus {
  border-color: #f4a259;
}

textarea {
  resize: vertical;
  font-family: Consolas, monospace;
  font-size: 12px;
}

textarea.prose {
  font-family: inherit;
  font-size: 13px;
  line-height: 1.5;
}

.check {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  cursor: pointer;
}

.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
}

button {
  padding: 7px 16px;
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

button:disabled {
  opacity: 0.6;
  cursor: default;
}

footer {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding: 12px 24px;
  background: #faf8f5;
  border-top: 1px solid #efe4d6;
}

.status {
  flex: 1;
  margin: 0;
  color: #4d7a3a;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status.error {
  color: #b5452f;
}
</style>
