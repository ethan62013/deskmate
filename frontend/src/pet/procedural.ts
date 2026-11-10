import * as THREE from 'three'
import { disposeTree, type PetKind, type PetModel, type PetState } from './types'

type V3 = [number, number, number]

function gradientMap() {
  const tex = new THREE.DataTexture(new Uint8Array([110, 190, 255]), 3, 1, THREE.RedFormat)
  tex.minFilter = THREE.NearestFilter
  tex.magFilter = THREE.NearestFilter
  tex.needsUpdate = true
  return tex
}

class Builder {
  private gradient = gradientMap()
  private materials = new Map<string, THREE.Material>()

  mat(color: number, opts: { basic?: boolean; opacity?: number } = {}) {
    const key = `${color}-${opts.basic}-${opts.opacity}`
    let m = this.materials.get(key)
    if (!m) {
      const transparent = opts.opacity !== undefined
      m = opts.basic
        ? new THREE.MeshBasicMaterial({ color, transparent, opacity: opts.opacity ?? 1 })
        : new THREE.MeshToonMaterial({ color, gradientMap: this.gradient, transparent, opacity: opts.opacity ?? 1 })
      this.materials.set(key, m)
    }
    return m
  }

  ball(parent: THREE.Object3D, r: number, color: number, pos: V3, scale: V3 = [1, 1, 1], opts = {}) {
    const mesh = new THREE.Mesh(new THREE.SphereGeometry(r, 32, 20), this.mat(color, opts))
    mesh.position.set(...pos)
    mesh.scale.set(...scale)
    parent.add(mesh)
    return mesh
  }

  capsule(parent: THREE.Object3D, r: number, len: number, color: number, pos: V3) {
    const mesh = new THREE.Mesh(new THREE.CapsuleGeometry(r, len, 6, 16), this.mat(color))
    mesh.position.set(...pos)
    parent.add(mesh)
    return mesh
  }

  cone(parent: THREE.Object3D, r: number, h: number, color: number, pos: V3, rot: V3 = [0, 0, 0]) {
    const mesh = new THREE.Mesh(new THREE.ConeGeometry(r, h, 16), this.mat(color))
    mesh.position.set(...pos)
    mesh.rotation.set(...rot)
    parent.add(mesh)
    return mesh
  }

  pivot(parent: THREE.Object3D, pos: V3) {
    const g = new THREE.Group()
    g.position.set(...pos)
    parent.add(g)
    return g
  }

  eye(parent: THREE.Object3D, r: number, pos: V3) {
    const eye = this.ball(parent, r, 0x1d1b2b, pos, [1, 1.15, 0.55])
    this.ball(eye, r * 0.34, 0xffffff, [r * 0.35, r * 0.45, r * 0.7], [1, 1, 1], { basic: true })
    return eye
  }
}

interface Rig {
  pose: THREE.Group
  body: THREE.Object3D
  head: THREE.Object3D
  eyes: THREE.Object3D[]
  mouth: THREE.Object3D
  mouthMode: 'scale' | 'rotate'
  legs: THREE.Object3D[] // front-left, front-right, back-left, back-right (or left, right for birds)
  tail: THREE.Object3D[]
  ears: THREE.Object3D[]
  wings: THREE.Object3D[]
  tongue?: THREE.Object3D
  bird: boolean
}

interface QuadStyle {
  fur: number
  light: number
  ear: 'pointy' | 'floppy'
  earColor: number
  nose: number
  tail: 'long' | 'short'
  muzzle: number
  tongue: boolean
}

function buildQuadruped(b: Builder, s: QuadStyle): Rig {
  const pose = new THREE.Group()

  const body = b.pivot(pose, [0, 0.47, -0.05])
  b.ball(body, 0.4, s.fur, [0, 0, 0], [1, 0.82, 1.1])
  b.ball(body, 0.29, s.light, [0, -0.05, 0.25], [1, 0.95, 0.55])

  const head = b.pivot(pose, [0, 0.88, 0.16])
  b.ball(head, 0.4, s.fur, [0, 0, 0], [1.12, 0.95, 1])
  b.ball(head, 0.17, s.light, [0, -0.1, 0.32], [1.35, 0.8, s.muzzle])
  b.ball(head, 0.045, s.nose, [0, -0.04, 0.33 + 0.13 * s.muzzle], [1.3, 0.9, 0.8])
  const mouth = b.ball(head, 0.06, 0x5a2630, [0, -0.17, 0.3 + 0.11 * s.muzzle], [1.2, 0.25, 0.6])
  let tongue: THREE.Object3D | undefined
  if (s.tongue) {
    tongue = b.ball(head, 0.05, 0xff7b8f, [0, -0.21, 0.31 + 0.11 * s.muzzle], [0.9, 0.5, 0.6])
  }
  const eyes = [b.eye(head, 0.075, [-0.16, 0.05, 0.35]), b.eye(head, 0.075, [0.16, 0.05, 0.35])]
  for (const x of [-0.27, 0.27]) {
    b.ball(head, 0.06, 0xff8fa3, [x, -0.09, 0.3], [1.3, 0.6, 0.3], { opacity: 0.55 })
  }

  const ears: THREE.Object3D[] = []
  for (const side of [-1, 1]) {
    if (s.ear === 'pointy') {
      const ear = b.pivot(head, [0.23 * side, 0.27, -0.02])
      ear.rotation.z = -0.35 * side
      b.cone(ear, 0.13, 0.26, s.earColor, [0, 0.1, 0])
      b.cone(ear, 0.075, 0.15, 0xffa5b4, [0, 0.07, 0.05])
      ears.push(ear)
    } else {
      const ear = b.pivot(head, [0.3 * side, 0.2, -0.02])
      ear.rotation.z = 0.3 * side
      b.ball(ear, 0.14, s.earColor, [0, -0.15, 0], [0.6, 1.3, 0.35])
      ears.push(ear)
    }
  }

  const legs: THREE.Object3D[] = []
  for (const [x, z] of [[-0.2, 0.18], [0.2, 0.18], [-0.2, -0.24], [0.2, -0.24]] as const) {
    const leg = b.pivot(pose, [x, 0.32, z])
    b.capsule(leg, 0.095, 0.14, s.fur, [0, -0.15, 0])
    b.ball(leg, 0.105, s.light, [0, -0.27, 0.02], [1, 0.6, 1.2])
    legs.push(leg)
  }

  const tail: THREE.Object3D[] = []
  if (s.tail === 'long') {
    let parent: THREE.Object3D = b.pivot(pose, [0, 0.5, -0.47])
    parent.rotation.x = -0.9
    for (let i = 0; i < 4; i++) {
      tail.push(parent)
      b.capsule(parent, 0.06, 0.12, i === 3 ? s.light : s.fur, [0, 0.09, 0])
      const next = b.pivot(parent, [0, 0.17, 0])
      next.rotation.x = 0.28
      parent = next
    }
  } else {
    const t = b.pivot(pose, [0, 0.56, -0.46])
    t.rotation.x = -0.9
    b.capsule(t, 0.08, 0.1, s.fur, [0, 0.1, 0])
    tail.push(t)
  }

  return { pose, body, head, eyes, mouth, mouthMode: 'scale', legs, tail, ears, wings: [], tongue, bird: false }
}

function buildParrot(b: Builder): Rig {
  const pose = new THREE.Group()
  const green = 0x3fca6f
  const dark = 0x23a05a

  const body = b.pivot(pose, [0, 0.62, 0])
  b.ball(body, 0.36, green, [0, 0, 0], [0.95, 1.2, 0.9])
  b.ball(body, 0.27, 0xffd34d, [0, -0.07, 0.18], [0.9, 1.1, 0.55])

  const head = b.pivot(pose, [0, 1.08, 0.06])
  b.ball(head, 0.31, green, [0, 0, 0], [1.05, 1, 1])
  b.ball(head, 0.19, 0xfff3df, [0, 0.0, 0.2], [1.35, 1, 0.5])
  const eyes = [b.eye(head, 0.062, [-0.13, 0.05, 0.27]), b.eye(head, 0.062, [0.13, 0.05, 0.27])]
  for (const x of [-0.21, 0.21]) {
    b.ball(head, 0.05, 0xff8a80, [x, -0.07, 0.22], [1.2, 0.8, 0.4], { opacity: 0.7 })
  }
  b.cone(head, 0.085, 0.17, 0xffa63a, [0, -0.06, 0.33], [Math.PI * 0.6, 0, 0])
  const mouth = b.pivot(head, [0, -0.1, 0.27])
  b.ball(mouth, 0.05, 0xe48a23, [0, -0.02, 0.03], [1, 0.6, 1])
  for (const [x, rz] of [[-0.06, 0.35], [0, 0], [0.06, -0.35]] as const) {
    b.cone(head, 0.04, 0.16, 0xff5d5d, [x, 0.33, -0.04], [-0.5, 0, rz])
  }

  const wings: THREE.Object3D[] = []
  for (const side of [-1, 1]) {
    const w = b.pivot(pose, [0.3 * side, 0.82, -0.02])
    b.ball(w, 0.22, dark, [0.02 * side, -0.16, -0.02], [0.35, 1, 0.7])
    b.ball(w, 0.12, 0x3a7bd5, [0.03 * side, -0.32, -0.06], [0.3, 0.8, 0.55])
    wings.push(w)
  }

  const tailPivot = b.pivot(pose, [0, 0.38, -0.24])
  tailPivot.rotation.x = 0.7
  b.ball(tailPivot, 0.1, 0x3a7bd5, [-0.04, 0, -0.22], [0.55, 0.25, 2])
  b.ball(tailPivot, 0.1, 0xff5d5d, [0.05, 0, -0.18], [0.5, 0.22, 1.7])

  const legs: THREE.Object3D[] = []
  for (const x of [-0.11, 0.11]) {
    const leg = b.pivot(pose, [x, 0.3, 0.05])
    b.capsule(leg, 0.03, 0.14, 0x9a8f86, [0, -0.12, 0])
    b.ball(leg, 0.06, 0x9a8f86, [0, -0.25, 0.04], [1, 0.4, 1.5])
    legs.push(leg)
  }

  return { pose, body, head, eyes, mouth, mouthMode: 'rotate', legs, tail: [tailPivot], ears: [], wings, bird: true }
}

const styles: Record<Exclude<PetKind, 'parrot'>, QuadStyle> = {
  cat: { fur: 0xf4a259, light: 0xfff4e6, ear: 'pointy', earColor: 0xf4a259, nose: 0xff8fa3, tail: 'long', muzzle: 0.8, tongue: false },
  dog: { fur: 0xe8b46a, light: 0xfff6ea, ear: 'floppy', earColor: 0xa0663a, nose: 0x2b2024, tail: 'short', muzzle: 1.15, tongue: true },
}

export function createProceduralPet(kind: PetKind): PetModel {
  const b = new Builder()
  const rig = kind === 'parrot' ? buildParrot(b) : buildQuadruped(b, styles[kind])
  const root = new THREE.Group()
  root.add(rig.pose)

  const rest = new Map<THREE.Object3D, THREE.Euler>()
  for (const o of [...rig.tail, ...rig.ears, ...rig.wings, ...rig.legs, rig.head]) rest.set(o, o.rotation.clone())

  let t = 0
  let nextBlink = 2
  let blinkT = -1

  const damp = (cur: number, target: number, dt: number, speed = 12) =>
    cur + (target - cur) * (1 - Math.exp(-dt * speed))
  const rot = (o: THREE.Object3D, x: number, y: number, z: number, dt: number, speed?: number) => {
    const r = rest.get(o)!
    o.rotation.x = damp(o.rotation.x, r.x + x, dt, speed)
    o.rotation.y = damp(o.rotation.y, r.y + y, dt, speed)
    o.rotation.z = damp(o.rotation.z, r.z + z, dt, speed)
  }

  function update(dt: number, state: PetState, st: number, look: number) {
    t += dt
    const breathe = Math.sin(t * 2.6)

    let poseY = 0
    let poseRX = 0
    let poseRY = 0
    let headX = Math.sin(t * 0.9) * 0.04
    let headY = ['idle', 'sit', 'talk'].includes(state) ? look * 0.45 : 0
    let headZ = Math.sin(t * 0.7) * 0.06
    let tailSwing = Math.sin(t * 2) * 0.3
    let tailLift = 0
    let wingFlap = 0
    let wingOnlyRight = false
    let eyeOpen = 1
    let mouthOpen = 0
    let bodyScaleY = 1 + breathe * 0.02
    const legRot = rig.legs.map(() => 0)

    switch (state) {
      case 'walk': {
        const phase = Math.sin(t * 9)
        if (rig.bird) {
          poseY = Math.abs(Math.sin(t * 7)) * 0.08
          wingFlap = Math.abs(Math.sin(t * 7)) * 0.25
        } else {
          legRot[0] = legRot[3] = phase * 0.6
          legRot[1] = legRot[2] = -phase * 0.6
          poseY = Math.abs(phase) * 0.035
        }
        tailSwing = Math.sin(t * 6) * 0.4
        break
      }
      case 'happy': {
        poseY = Math.max(0, Math.sin(st * Math.PI * 2.4)) * 0.28
        if (st < 0.8) {
          const k = st / 0.8
          poseRY = (k * k * (3 - 2 * k)) * Math.PI * 2
        }
        tailSwing = Math.sin(t * 18) * 0.6
        wingFlap = Math.abs(Math.sin(t * 18)) * 1.1
        eyeOpen = 0.4
        mouthOpen = 0.6
        break
      }
      case 'sit': {
        if (rig.bird) {
          bodyScaleY = 0.96 + breathe * 0.02
          poseY = -0.03
        } else {
          poseY = -0.07
          poseRX = -0.2
          legRot[0] = legRot[1] = 0.2
          legRot[2] = legRot[3] = -1.2
          tailSwing = Math.sin(t * 1.2) * 0.2
          tailLift = 0.5
        }
        break
      }
      case 'sleep': {
        poseY = rig.bird ? -0.05 : -0.13
        bodyScaleY = 0.88 + Math.sin(t * 1.4) * 0.04
        headX = 0.35
        headY = 0
        headZ = rig.bird ? 0 : 0.25
        eyeOpen = 0.08
        tailSwing = 0
        tailLift = 0.6
        if (!rig.bird) legRot.fill(-1.4)
        break
      }
      case 'talk': {
        mouthOpen = Math.sin(t * 14) * 0.5 + 0.5
        headX = Math.sin(t * 7) * 0.06
        tailSwing = Math.sin(t * 4) * 0.35
        break
      }
      case 'remind': {
        poseY = Math.abs(Math.sin(t * 6)) * 0.08
        mouthOpen = Math.sin(t * 12) * 0.5 + 0.5
        if (rig.bird) {
          wingFlap = 0.6 + Math.sin(t * 14) * 0.6
          wingOnlyRight = true
        } else {
          legRot[1] = -1.6 + Math.sin(t * 10) * 0.35
        }
        break
      }
    }

    blinkT = blinkT < 0 && t > nextBlink ? 0 : blinkT
    if (blinkT >= 0) {
      blinkT += dt
      if (blinkT > 0.14) {
        blinkT = -1
        nextBlink = t + 2.5 + Math.random() * 3
      } else {
        eyeOpen = Math.min(eyeOpen, 0.1)
      }
    }

    rig.pose.position.y = damp(rig.pose.position.y, poseY, dt, state === 'happy' ? 30 : 12)
    rig.pose.rotation.x = damp(rig.pose.rotation.x, poseRX, dt)
    rig.pose.rotation.y = state === 'happy' ? poseRY : damp(rig.pose.rotation.y % (Math.PI * 2), 0, dt)
    rig.body.scale.y = damp(rig.body.scale.y, bodyScaleY, dt)

    rot(rig.head, headX, headY, headZ, dt, 8)
    rig.legs.forEach((leg, i) => rot(leg, legRot[i], 0, 0, dt, 16))
    rig.tail.forEach((seg, i) => rot(seg, -tailLift * (i === 0 ? 1 : 0.2), 0, tailSwing * (i === 0 ? 1 : 0.6), dt, 14))
    rig.ears.forEach((ear, i) => rot(ear, 0, 0, (state === 'happy' ? Math.sin(t * 16) * 0.15 : 0) * (i ? -1 : 1), dt))
    rig.wings.forEach((w, i) => {
      const side = i === 0 ? -1 : 1
      const flap = wingOnlyRight && side < 0 ? 0 : wingFlap
      rot(w, 0, 0, flap * side, dt, 20)
    })
    for (const eye of rig.eyes) eye.scale.y = damp(eye.scale.y, 1.15 * eyeOpen, dt, 30)

    if (rig.mouthMode === 'scale') {
      rig.mouth.scale.y = damp(rig.mouth.scale.y, 0.25 + mouthOpen * 0.85, dt, 25)
    } else {
      rig.mouth.rotation.x = damp(rig.mouth.rotation.x, mouthOpen * 0.5, dt, 25)
    }
    if (rig.tongue) rig.tongue.visible = mouthOpen > 0.3 || state === 'happy'
  }

  return { root, update, dispose: () => disposeTree(root) }
}
