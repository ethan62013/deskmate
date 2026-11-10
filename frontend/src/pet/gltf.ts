import * as THREE from 'three'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { disposeTree, type PetModel, type PetState } from './types'

const PET_HEIGHT = 1.35

const clipCandidates: Record<PetState, string[]> = {
  idle: ['idle'],
  walk: ['walk', 'run'],
  happy: ['jump', 'happy', 'hit'],
  sit: ['sit', 'idle_alt'],
  sleep: ['sleep', 'sit', 'idle'],
  talk: ['call', 'eat', 'idle_alt', 'idle'],
  remind: ['call', 'jump'],
}

/** Loads /models/<kind>.glb; resolves to null when the file is missing or unreadable. */
export async function loadGltfPet(url: string): Promise<PetModel | null> {
  let gltf
  try {
    gltf = await new GLTFLoader().loadAsync(url)
  } catch {
    return null
  }

  const model = gltf.scene
  const lowDetail: THREE.Object3D[] = []
  model.traverse((o) => {
    if (/_LOD[1-9]$/i.test(o.name)) lowDetail.push(o)
  })
  for (const o of lowDetail) {
    o.removeFromParent()
    disposeTree(o)
  }

  const box = new THREE.Box3().setFromObject(model)
  const size = box.getSize(new THREE.Vector3())
  const scale = PET_HEIGHT / Math.max(size.y, 1e-3)
  model.scale.setScalar(scale)
  box.setFromObject(model)
  const center = box.getCenter(new THREE.Vector3())
  model.position.set(-center.x, -box.min.y, -center.z)

  const root = new THREE.Group()
  root.add(model)

  const mixer = new THREE.AnimationMixer(model)
  const clips = gltf.animations
  const findClip = (state: PetState) => {
    for (const name of clipCandidates[state]) {
      const exact = clips.find((c) => c.name.toLowerCase().split('|').pop() === name)
      if (exact) return exact
    }
    for (const name of clipCandidates[state]) {
      const loose = clips.find((c) => c.name.toLowerCase().includes(name))
      if (loose) return loose
    }
    return clips[0]
  }

  let current: THREE.AnimationAction | null = null
  let currentState: PetState | null = null

  function update(dt: number, state: PetState) {
    if (state !== currentState && clips.length) {
      currentState = state
      const next = mixer.clipAction(findClip(state))
      if (next !== current) {
        next.reset().fadeIn(0.25).play()
        current?.fadeOut(0.25)
        current = next
      }
    }
    mixer.update(dt)
  }

  return {
    root,
    update,
    dispose: () => {
      mixer.stopAllAction()
      disposeTree(root)
    },
  }
}
